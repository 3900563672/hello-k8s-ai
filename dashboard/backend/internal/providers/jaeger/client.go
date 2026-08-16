package jaeger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/httputil"
)

var (
	ErrDisabled      = errors.New("Jaeger provider is disabled")
	ErrInvalidQuery  = errors.New("invalid Jaeger trace query")
	ErrTraceNotFound = errors.New("Jaeger trace was not found")
)

type SearchRequest struct {
	Start       time.Time
	End         time.Time
	Service     string
	Operation   string
	Tenant      string
	Model       string
	Instance    string
	MinDuration time.Duration
	MaxDuration time.Duration
	Limit       int
}

type Client struct {
	baseURL   *url.URL
	http      *http.Client
	enabled   bool
	maxWindow time.Duration
}

func New(cfg config.ProviderConfig) (*Client, error) {
	baseURL, err := httputil.ParseBaseURL(cfg.URL, "Jaeger")
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL:   baseURL,
		http:      httputil.NewClient(cfg.Timeout),
		enabled:   cfg.Enabled,
		maxWindow: cfg.MaxWindow,
	}, nil
}

func (client *Client) Search(ctx context.Context, request SearchRequest) ([]model.TraceSummary, error) {
	if !client.enabled {
		return nil, ErrDisabled
	}
	request = normalizeSearch(request)
	if err := client.validate(request); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidQuery, err)
	}

	services := []string{request.Service}
	if request.Service == "" {
		var err error
		services, err = client.services(ctx)
		if err != nil {
			return nil, err
		}
		filtered := services[:0]
		for _, service := range services {
			if strings.HasPrefix(service, "hello-k8s-ai") {
				filtered = append(filtered, service)
			}
		}
		services = filtered
		if len(services) > 4 {
			services = services[:4]
		}
	}

	byID := make(map[string]model.TraceSummary)
	for _, service := range services {
		traces, err := client.searchService(ctx, request, service)
		if err != nil {
			return nil, err
		}
		for _, trace := range traces {
			byID[trace.TraceID] = summarize(trace)
		}
	}
	result := make([]model.TraceSummary, 0, len(byID))
	for _, trace := range byID {
		result = append(result, trace)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StartTime.After(result[j].StartTime) })
	if len(result) > request.Limit {
		result = result[:request.Limit]
	}
	return result, nil
}

func (client *Client) Trace(ctx context.Context, traceID string) (model.TraceDetail, error) {
	if !client.enabled {
		return model.TraceDetail{}, ErrDisabled
	}
	if traceID == "" || len(traceID) > 64 || strings.ContainsAny(traceID, "/?&#") {
		return model.TraceDetail{}, fmt.Errorf("%w: invalid traceId", ErrInvalidQuery)
	}
	endpoint := httputil.Resolve(client.baseURL, "/api/traces/"+url.PathEscape(traceID))
	var response traceResponse
	if err := httputil.GetJSON(ctx, client.http, endpoint, &response, "Jaeger", 32<<20); err != nil {
		return model.TraceDetail{}, err
	}
	if len(response.Data) == 0 {
		return model.TraceDetail{}, fmt.Errorf("%w: %s", ErrTraceNotFound, traceID)
	}
	return detail(response.Data[0]), nil
}

func (client *Client) Health(ctx context.Context) error {
	if !client.enabled {
		return ErrDisabled
	}
	_, err := client.services(ctx)
	return err
}

func (client *Client) Enabled() bool {
	return client.enabled
}

func (client *Client) searchService(ctx context.Context, request SearchRequest, service string) ([]legacyTrace, error) {
	endpoint := httputil.Resolve(client.baseURL, "/api/traces")
	query := endpoint.Query()
	query.Set("service", service)
	query.Set("start", strconv.FormatInt(request.Start.UnixMicro(), 10))
	query.Set("end", strconv.FormatInt(request.End.UnixMicro(), 10))
	query.Set("limit", strconv.Itoa(request.Limit))
	if request.Operation != "" {
		query.Set("operation", request.Operation)
	}
	if request.MinDuration > 0 {
		query.Set("minDuration", formatDuration(request.MinDuration))
	}
	if request.MaxDuration > 0 {
		query.Set("maxDuration", formatDuration(request.MaxDuration))
	}
	tags := map[string]string{}
	if request.Tenant != "" {
		tags["platform.tenant.name"] = request.Tenant
	}
	if request.Model != "" {
		tags["platform.model.name"] = request.Model
	}
	if request.Instance != "" {
		tags["platform.simulator_instance.name"] = request.Instance
	}
	if len(tags) > 0 {
		encoded, _ := json.Marshal(tags)
		query.Set("tags", string(encoded))
	}
	endpoint.RawQuery = query.Encode()
	var response traceResponse
	if err := httputil.GetJSON(ctx, client.http, endpoint, &response, "Jaeger", 32<<20); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (client *Client) services(ctx context.Context) ([]string, error) {
	endpoint := httputil.Resolve(client.baseURL, "/api/services")
	var response struct {
		Data   []string `json:"data"`
		Errors []any    `json:"errors"`
	}
	if err := httputil.GetJSON(ctx, client.http, endpoint, &response, "Jaeger", 32<<20); err != nil {
		return nil, err
	}
	sort.Strings(response.Data)
	return response.Data, nil
}

func (client *Client) validate(request SearchRequest) error {
	if !request.Start.Before(request.End) {
		return errors.New("start must be before end")
	}
	if request.End.Sub(request.Start) > client.maxWindow {
		return fmt.Errorf("trace window exceeds maximum %s", client.maxWindow)
	}
	if request.Limit < 1 || request.Limit > 100 {
		return errors.New("trace limit must be between 1 and 100")
	}
	if request.MinDuration < 0 || request.MaxDuration < 0 {
		return errors.New("trace durations must not be negative")
	}
	if request.MaxDuration > 0 && request.MinDuration > request.MaxDuration {
		return errors.New("minDuration must not exceed maxDuration")
	}
	for name, value := range map[string]string{
		"service": request.Service, "operation": request.Operation,
		"tenant": request.Tenant, "model": request.Model, "instance": request.Instance,
	} {
		if len(value) > 253 || strings.ContainsAny(value, "\x00\r\n") {
			return fmt.Errorf("invalid %s filter", name)
		}
	}
	return nil
}

func normalizeSearch(request SearchRequest) SearchRequest {
	if request.End.IsZero() {
		request.End = time.Now().UTC()
	}
	if request.Start.IsZero() {
		request.Start = request.End.Add(-15 * time.Minute)
	}
	request.Start = request.Start.UTC()
	request.End = request.End.UTC()
	if request.Limit <= 0 {
		request.Limit = 20
	}
	return request
}

type traceResponse struct {
	Data   []legacyTrace `json:"data"`
	Errors []any         `json:"errors"`
}

type legacyTrace struct {
	TraceID   string                   `json:"traceID"`
	Spans     []legacySpan             `json:"spans"`
	Processes map[string]legacyProcess `json:"processes"`
}

type legacySpan struct {
	TraceID       string            `json:"traceID"`
	SpanID        string            `json:"spanID"`
	OperationName string            `json:"operationName"`
	References    []legacyReference `json:"references"`
	StartTime     int64             `json:"startTime"`
	Duration      int64             `json:"duration"`
	Tags          []legacyTag       `json:"tags"`
	Logs          []legacyLog       `json:"logs"`
	ProcessID     string            `json:"processID"`
}

type legacyReference struct {
	RefType string `json:"refType"`
	TraceID string `json:"traceID"`
	SpanID  string `json:"spanID"`
}

type legacyProcess struct {
	ServiceName string      `json:"serviceName"`
	Tags        []legacyTag `json:"tags"`
}

type legacyTag struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type legacyLog struct {
	Timestamp int64       `json:"timestamp"`
	Fields    []legacyTag `json:"fields"`
}

func summarize(trace legacyTrace) model.TraceSummary {
	root := rootSpan(trace.Spans)
	attributes := mergedAttributes(root, trace.Processes[root.ProcessID])
	errors := 0
	for _, span := range trace.Spans {
		if spanStatus(span.Tags) == "error" {
			errors++
		}
		for key, value := range mergedAttributes(span, trace.Processes[span.ProcessID]) {
			if _, exists := attributes[key]; !exists {
				attributes[key] = value
			}
		}
	}
	return model.TraceSummary{
		TraceID:        trace.TraceID,
		RootService:    trace.Processes[root.ProcessID].ServiceName,
		RootOperation:  root.OperationName,
		StartTime:      microTime(root.StartTime),
		DurationMs:     float64(root.Duration) / 1000,
		SpanCount:      len(trace.Spans),
		ErrorSpanCount: errors,
		Entities: map[string]string{
			"tenant":            stringAttribute(attributes, "platform.tenant.name"),
			"model":             stringAttribute(attributes, "platform.model.name"),
			"simulatorInstance": stringAttribute(attributes, "platform.simulator_instance.name"),
		},
	}
}

func detail(trace legacyTrace) model.TraceDetail {
	spans := make([]model.Span, 0, len(trace.Spans))
	for _, span := range trace.Spans {
		process := trace.Processes[span.ProcessID]
		attributes := mergedAttributes(span, process)
		events := make([]model.SpanEvent, 0, len(span.Logs))
		for _, log := range span.Logs {
			fields := tagsToMap(log.Fields)
			name := stringAttribute(fields, "event")
			if name == "" {
				name = "log"
			}
			events = append(events, model.SpanEvent{Name: name, Time: microTime(log.Timestamp), Attributes: fields})
		}
		spans = append(spans, model.Span{
			SpanID:       span.SpanID,
			ParentSpanID: parentSpanID(span.References),
			Service:      process.ServiceName,
			Operation:    span.OperationName,
			StartTime:    microTime(span.StartTime),
			DurationMs:   float64(span.Duration) / 1000,
			Status:       spanStatus(span.Tags),
			Attributes:   attributes,
			Events:       events,
		})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].StartTime.Before(spans[j].StartTime) })
	return model.TraceDetail{TraceID: trace.TraceID, Spans: spans, EntityLinks: []model.ResourceRef{}}
}

func rootSpan(spans []legacySpan) legacySpan {
	if len(spans) == 0 {
		return legacySpan{}
	}
	root := spans[0]
	for _, span := range spans {
		if parentSpanID(span.References) == "" && span.StartTime <= root.StartTime {
			root = span
		}
	}
	return root
}

func parentSpanID(references []legacyReference) string {
	for _, reference := range references {
		if reference.RefType == "CHILD_OF" {
			return reference.SpanID
		}
	}
	return ""
}

func mergedAttributes(span legacySpan, process legacyProcess) map[string]any {
	result := tagsToMap(process.Tags)
	for key, value := range tagsToMap(span.Tags) {
		result[key] = value
	}
	return result
}

func tagsToMap(tags []legacyTag) map[string]any {
	result := make(map[string]any, len(tags))
	for _, tag := range tags {
		result[tag.Key] = tag.Value
	}
	return result
}

func spanStatus(tags []legacyTag) string {
	attributes := tagsToMap(tags)
	if value, exists := attributes["error"]; exists {
		if boolean, ok := value.(bool); ok && boolean {
			return "error"
		}
	}
	if strings.EqualFold(stringAttribute(attributes, "otel.status_code"), "ERROR") {
		return "error"
	}
	return "ok"
}

func stringAttribute(attributes map[string]any, key string) string {
	value, exists := attributes[key]
	if !exists || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func formatDuration(value time.Duration) string {
	return strconv.FormatInt(value.Microseconds(), 10) + "us"
}

func microTime(value int64) time.Time {
	return time.Unix(0, value*int64(time.Microsecond)).UTC()
}
