package prometheus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/httputil"
)

var (
	ErrDisabled     = errors.New("Prometheus provider is disabled")
	ErrInvalidQuery = errors.New("invalid Prometheus metric query")
)

type Query struct {
	MetricID string
	Start    time.Time
	End      time.Time
	Step     time.Duration
	Tenant   string
	Model    string
	Instance string
	Node     string
}

type metricDefinition struct {
	unit  string
	build func(map[string]string) string
}

type cacheEntry struct {
	expires time.Time
	value   model.MetricResult
}

type Client struct {
	baseURL   *url.URL
	http      *http.Client
	enabled   bool
	maxWindow time.Duration
	cacheTTL  time.Duration
	mu        sync.Mutex
	cache     map[string]cacheEntry
}

func New(cfg config.ProviderConfig) (*Client, error) {
	baseURL, err := httputil.ParseBaseURL(cfg.URL, "Prometheus")
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL:   baseURL,
		http:      httputil.NewClient(cfg.Timeout),
		enabled:   cfg.Enabled,
		maxWindow: cfg.MaxWindow,
		cacheTTL:  cfg.CacheTTL,
		cache:     make(map[string]cacheEntry),
	}, nil
}

func (client *Client) QueryRange(ctx context.Context, query Query) (model.MetricResult, error) {
	if !client.enabled {
		return model.MetricResult{}, ErrDisabled
	}
	definition, exists := metricCatalog()[query.MetricID]
	if !exists {
		return model.MetricResult{}, fmt.Errorf("%w: unknown metricId %q", ErrInvalidQuery, query.MetricID)
	}
	query = normalizeQuery(query)
	if err := client.validate(query); err != nil {
		return model.MetricResult{}, fmt.Errorf("%w: %v", ErrInvalidQuery, err)
	}

	filters := map[string]string{
		"tenant":             query.Tenant,
		"model":              query.Model,
		"simulator_instance": query.Instance,
		"node":               query.Node,
	}
	promQL := definition.build(filters)
	cacheKey := strings.Join([]string{
		query.MetricID,
		query.Start.Format(time.RFC3339Nano),
		query.End.Format(time.RFC3339Nano),
		query.Step.String(),
		promQL,
	}, "|")
	if value, ok := client.cached(cacheKey); ok {
		return value, nil
	}

	endpoint := httputil.Resolve(client.baseURL, "/api/v1/query_range")
	parameters := endpoint.Query()
	parameters.Set("query", promQL)
	parameters.Set("start", formatPromTime(query.Start))
	parameters.Set("end", formatPromTime(query.End))
	parameters.Set("step", strconv.FormatFloat(query.Step.Seconds(), 'f', -1, 64))
	endpoint.RawQuery = parameters.Encode()

	var response apiResponse
	queriedAt := time.Now().UTC()
	if err := httputil.GetJSON(ctx, client.http, endpoint, &response, "Prometheus", 16<<20); err != nil {
		return model.MetricResult{}, err
	}
	if response.Status != "success" {
		return model.MetricResult{}, fmt.Errorf("Prometheus query failed: %s: %s", response.ErrorType, response.Error)
	}
	series, err := parseSeries(response.Data.ResultType, response.Data.Result)
	if err != nil {
		return model.MetricResult{}, fmt.Errorf("parse Prometheus result: %w", err)
	}
	result := model.MetricResult{
		MetricID:    query.MetricID,
		Unit:        definition.unit,
		Start:       query.Start,
		End:         query.End,
		StepSeconds: int64(query.Step.Seconds()),
		Series:      series,
		ResultType:  response.Data.ResultType,
		Warnings:    append([]string{}, response.Warnings...),
		QueriedAt:   queriedAt,
	}
	client.putCache(cacheKey, result)
	return result, nil
}

func (client *Client) Health(ctx context.Context) error {
	if !client.enabled {
		return ErrDisabled
	}
	endpoint := httputil.Resolve(client.baseURL, "/api/v1/query")
	parameters := endpoint.Query()
	parameters.Set("query", "1")
	endpoint.RawQuery = parameters.Encode()
	var response apiResponse
	if err := httputil.GetJSON(ctx, client.http, endpoint, &response, "Prometheus", 16<<20); err != nil {
		return err
	}
	if response.Status != "success" {
		return fmt.Errorf("Prometheus health query returned %s", response.Status)
	}
	return nil
}

func (client *Client) Enabled() bool {
	return client.enabled
}

func (client *Client) Catalog() []map[string]string {
	catalog := metricCatalog()
	keys := make([]string, 0, len(catalog))
	for key := range catalog {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]map[string]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, map[string]string{"metricId": key, "unit": catalog[key].unit})
	}
	return result
}

func (client *Client) validate(query Query) error {
	if query.Start.IsZero() || query.End.IsZero() {
		return errors.New("start and end are required")
	}
	if !query.Start.Before(query.End) {
		return errors.New("start must be before end")
	}
	if query.End.Sub(query.Start) > client.maxWindow {
		return fmt.Errorf("metric window exceeds maximum %s", client.maxWindow)
	}
	if query.Step < 5*time.Second || query.Step > time.Hour {
		return errors.New("step must be between 5s and 1h")
	}
	for name, value := range map[string]string{
		"tenant": query.Tenant, "model": query.Model, "instance": query.Instance, "node": query.Node,
	} {
		if len(value) > 253 || strings.ContainsAny(value, "\x00\r\n") {
			return fmt.Errorf("invalid %s filter", name)
		}
	}
	return nil
}

func normalizeQuery(query Query) Query {
	now := time.Now().UTC()
	if query.End.IsZero() {
		query.End = now
	}
	if query.Start.IsZero() {
		query.Start = query.End.Add(-15 * time.Minute)
	}
	query.Start = query.Start.UTC()
	query.End = query.End.UTC()
	if query.Step <= 0 {
		seconds := math.Ceil(query.End.Sub(query.Start).Seconds() / 120)
		query.Step = time.Duration(max(5, int(seconds))) * time.Second
	}
	return query
}

func metricCatalog() map[string]metricDefinition {
	return map[string]metricDefinition{
		"simulator.ttft": {
			unit: "ms",
			build: func(filters map[string]string) string {
				return "max by (tenant, model, simulator_instance) (" + selector("hello_k8s_ai_simulator_ttft_seconds", filters) + ") * 1000"
			},
		},
		"simulator.queue": {
			unit: "requests",
			build: func(filters map[string]string) string {
				return "max by (tenant, model, simulator_instance) (" + selector("hello_k8s_ai_simulator_queue_depth", filters) + ")"
			},
		},
		"simulator.qps": {
			unit: "requests/s",
			build: func(filters map[string]string) string {
				return "max by (tenant, model, simulator_instance) (" + selector("hello_k8s_ai_simulator_assigned_qps", filters) + ")"
			},
		},
		"simulator.errorRate": {
			unit: "ratio",
			build: func(filters map[string]string) string {
				errorFilters := cloneFilters(filters)
				errorFilters["outcome"] = "error"
				return "sum(rate(" + selector("hello_k8s_ai_simulator_ticks_total", errorFilters) + "[5m])) / clamp_min(sum(rate(" + selector("hello_k8s_ai_simulator_ticks_total", filters) + "[5m])), 1e-9)"
			},
		},
		"simulator.tickLatency": {
			unit: "ms",
			build: func(filters map[string]string) string {
				return "histogram_quantile(0.95, sum by (le) (rate(" + selector("hello_k8s_ai_simulator_tick_duration_seconds_bucket", filters) + "[5m]))) * 1000"
			},
		},
		"simulator.timeScale": {
			unit: "x",
			build: func(filters map[string]string) string {
				return "max by (tenant, model, simulator_instance) (" + selector("hello_k8s_ai_simulator_time_scale", filters) + ")"
			},
		},
		"controller.errorRate": {
			unit: "ratio",
			build: func(map[string]string) string {
				return "hello_k8s_ai:controller_reconcile_error_ratio:rate5m"
			},
		},
		"controller.reconcileLatency": {
			unit: "ms",
			build: func(map[string]string) string {
				return "hello_k8s_ai:controller_reconcile_duration_seconds:p95_5m * 1000"
			},
		},
		"worker.gpuUsed": {
			unit: "gpu-units",
			build: func(filters map[string]string) string {
				return selector("hello_k8s_ai_worker_node_gpu_units_used", map[string]string{"node": filters["node"]})
			},
		},
	}
}

func selector(metric string, filters map[string]string) string {
	keys := make([]string, 0, len(filters))
	for key, value := range filters {
		if value != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return metric
	}
	matchers := make([]string, 0, len(keys))
	for _, key := range keys {
		matchers = append(matchers, key+"="+strconv.Quote(filters[key]))
	}
	return metric + "{" + strings.Join(matchers, ",") + "}"
}

func cloneFilters(filters map[string]string) map[string]string {
	result := make(map[string]string, len(filters)+1)
	for key, value := range filters {
		result[key] = value
	}
	return result
}

type apiResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string          `json:"resultType"`
		Result     json.RawMessage `json:"result"`
	} `json:"data"`
	ErrorType string   `json:"errorType"`
	Error     string   `json:"error"`
	Warnings  []string `json:"warnings"`
}

type rawSeries struct {
	Metric map[string]string   `json:"metric"`
	Values [][]json.RawMessage `json:"values"`
	Value  []json.RawMessage   `json:"value"`
}

func parseSeries(resultType string, raw json.RawMessage) ([]model.MetricSeries, error) {
	var results []rawSeries
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, err
	}
	series := make([]model.MetricSeries, 0, len(results))
	for _, result := range results {
		pairs := result.Values
		if len(pairs) == 0 && len(result.Value) == 2 {
			pairs = [][]json.RawMessage{result.Value}
		}
		points := make([]model.MetricPoint, 0, len(pairs))
		for _, pair := range pairs {
			if len(pair) != 2 {
				continue
			}
			var timestamp float64
			var rawValue string
			if err := json.Unmarshal(pair[0], &timestamp); err != nil {
				continue
			}
			if err := json.Unmarshal(pair[1], &rawValue); err != nil {
				continue
			}
			value, err := strconv.ParseFloat(rawValue, 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
				continue
			}
			seconds, fraction := math.Modf(timestamp)
			points = append(points, model.MetricPoint{
				Time:  time.Unix(int64(seconds), int64(fraction*float64(time.Second))).UTC(),
				Value: value,
			})
		}
		labels := result.Metric
		if labels == nil {
			labels = map[string]string{}
		}
		series = append(series, model.MetricSeries{Labels: labels, Points: points})
	}
	return series, nil
}

func formatPromTime(value time.Time) string {
	return strconv.FormatFloat(float64(value.UnixNano())/float64(time.Second), 'f', 3, 64)
}

func (client *Client) cached(key string) (model.MetricResult, bool) {
	client.mu.Lock()
	defer client.mu.Unlock()
	entry, ok := client.cache[key]
	if !ok || time.Now().After(entry.expires) {
		delete(client.cache, key)
		return model.MetricResult{}, false
	}
	return entry.value, true
}

func (client *Client) putCache(key string, value model.MetricResult) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.cache) > 256 {
		client.cache = make(map[string]cacheEntry)
	}
	client.cache[key] = cacheEntry{expires: time.Now().Add(client.cacheTTL), value: value}
}
