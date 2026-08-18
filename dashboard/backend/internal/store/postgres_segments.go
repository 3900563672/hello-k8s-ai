package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/jackc/pgx/v5"
)

// ListResourceEvents 返回 since 之后的资源事件（采样器用它做事件分类与高保真触发）。
func (database *Postgres) ListResourceEvents(ctx context.Context, since time.Time, limit int) ([]model.ResourceChange, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := database.pool.Query(ctx, `
		SELECT event_id, occurred_at, operation, api_version, kind, namespace, name, uid,
		       resource_version, generation, payload
		FROM resource_events
		WHERE occurred_at >= $1
		ORDER BY occurred_at ASC, sequence ASC
		LIMIT $2`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("list resource events: %w", err)
	}
	defer rows.Close()
	var changes []model.ResourceChange
	for rows.Next() {
		var change model.ResourceChange
		if err := rows.Scan(
			&change.EventID, &change.OccurredAt, &change.Operation, &change.Ref.APIVersion,
			&change.Ref.Kind, &change.Ref.Namespace, &change.Ref.Name, &change.Ref.UID,
			&change.ResourceVersion, &change.Generation, &change.Payload,
		); err != nil {
			return nil, fmt.Errorf("scan resource event: %w", err)
		}
		changes = append(changes, change)
	}
	return changes, rows.Err()
}

func (database *Postgres) CreateSegment(ctx context.Context, record SegmentRecord) error {
	if record.SegmentID == "" {
		record.SegmentID = newID("segment")
	}
	configSnapshot := record.ConfigSnapshot
	if len(configSnapshot) == 0 {
		configSnapshot = json.RawMessage(`{}`)
	}
	_, err := database.pool.Exec(ctx, `
		INSERT INTO segments (segment_id, tenant, name, status, config_snapshot)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (segment_id) DO NOTHING`,
		record.SegmentID, record.Tenant, record.Name, record.Status, configSnapshot,
	)
	if err != nil {
		return fmt.Errorf("create segment: %w", err)
	}
	return nil
}

// UpdateSegmentLifecycle 推进切面生命周期：pending→running 时写入起点快照，
// running→completed/failed 时写入终点快照、摘要与结束原因。
func (database *Postgres) UpdateSegmentLifecycle(
	ctx context.Context,
	segmentID, status, reason string,
	endSnapshot, summary json.RawMessage,
) error {
	var snapshotColumn, snapshotValue string
	if status == string(model.SegmentRunning) {
		snapshotColumn = "start_snapshot"
		snapshotValue = "COALESCE(start_snapshot, $3)"
	} else {
		snapshotColumn = "end_snapshot"
		snapshotValue = "$3"
	}
	if len(endSnapshot) == 0 {
		endSnapshot = nil
	}
	_, err := database.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE segments
		SET status = $1,
		    reason = $2,
		    %s = %s,
		    summary = COALESCE($4, summary),
		    started_at = COALESCE(started_at, CASE WHEN $1 = 'running' THEN clock_timestamp() END),
		    ended_at = CASE WHEN $1 IN ('completed','failed') THEN clock_timestamp() END,
		    updated_at = clock_timestamp()
		WHERE segment_id = $5`, snapshotColumn, snapshotValue),
		status, reason, endSnapshot, summary, segmentID,
	)
	if err != nil {
		return fmt.Errorf("update segment lifecycle: %w", err)
	}
	return nil
}

func (database *Postgres) ListSegments(ctx context.Context, limit int, status string) ([]SegmentRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT segment_id, tenant, name, status, reason, config_snapshot,
		       start_snapshot, end_snapshot, summary, started_at, ended_at, created_at, updated_at
		FROM segments`
	var args []any
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC LIMIT ` + fmt.Sprintf("%d", limit)
	rows, err := database.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list segments: %w", err)
	}
	defer rows.Close()
	var records []SegmentRecord
	for rows.Next() {
		record, err := scanSegment(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	return records, rows.Err()
}

func (database *Postgres) GetSegment(ctx context.Context, segmentID string) (*SegmentRecord, error) {
	row := database.pool.QueryRow(ctx, `
		SELECT segment_id, tenant, name, status, reason, config_snapshot,
		       start_snapshot, end_snapshot, summary, started_at, ended_at, created_at, updated_at
		FROM segments WHERE segment_id = $1`, segmentID)
	record, err := scanSegment(row)
	if err != nil {
		return nil, fmt.Errorf("get segment %s: %w", segmentID, err)
	}
	return record, nil
}

func (database *Postgres) RecordSegmentEvent(ctx context.Context, event SegmentEvent) error {
	if event.EventID == "" {
		event.EventID = newID("seg-event")
	}
	payload := event.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	_, err := database.pool.Exec(ctx, `
		INSERT INTO segment_events (event_id, segment_id, event_type, occurred_at, entity, severity, payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (event_id) DO NOTHING`,
		event.EventID, event.SegmentID, event.EventType, event.OccurredAt,
		event.Entity, event.Severity, payload,
	)
	if err != nil {
		return fmt.Errorf("record segment event: %w", err)
	}
	return nil
}

func (database *Postgres) AppendSegmentMetrics(ctx context.Context, segmentID string, buckets []MetricBucket) error {
	if len(buckets) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, bucket := range buckets {
		batch.Queue(`
			INSERT INTO segment_metrics (
				metric_id, segment_id, metric_name, bucket_start, bucket_end, min, max, avg, p95
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (segment_id, metric_name, bucket_start) DO UPDATE SET
				bucket_end=EXCLUDED.bucket_end,
				min=EXCLUDED.min,
				max=EXCLUDED.max,
				avg=EXCLUDED.avg,
				p95=EXCLUDED.p95`,
			newID("seg-metric"), segmentID, bucket.MetricName,
			bucket.BucketStart, bucket.BucketEnd, bucket.Min, bucket.Max, bucket.Avg, bucket.P95,
		)
	}
	results := database.pool.SendBatch(ctx, batch)
	for range buckets {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("append segment metrics: %w", err)
		}
	}
	return results.Close()
}

func (database *Postgres) LinkSegmentTraces(ctx context.Context, segmentID string, traceIDs []string) error {
	if len(traceIDs) == 0 {
		return nil
	}
	for _, traceID := range traceIDs {
		if _, err := database.pool.Exec(ctx, `
			UPDATE trace_index SET segment_id = $1 WHERE trace_id = $2 AND segment_id IS NULL`,
			segmentID, traceID); err != nil {
			return fmt.Errorf("link segment trace %s: %w", traceID, err)
		}
	}
	return nil
}

func (database *Postgres) ListSegmentEvents(ctx context.Context, segmentID string, limit int) ([]SegmentEvent, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := database.pool.Query(ctx, `
		SELECT event_id, segment_id, event_type, occurred_at, entity, severity, payload
		FROM segment_events WHERE segment_id = $1
		ORDER BY occurred_at ASC
		LIMIT $2`, segmentID, limit)
	if err != nil {
		return nil, fmt.Errorf("list segment events: %w", err)
	}
	defer rows.Close()
	var events []SegmentEvent
	for rows.Next() {
		var event SegmentEvent
		if err := rows.Scan(&event.EventID, &event.SegmentID, &event.EventType,
			&event.OccurredAt, &event.Entity, &event.Severity, &event.Payload); err != nil {
			return nil, fmt.Errorf("scan segment event: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (database *Postgres) ListSegmentMetrics(ctx context.Context, segmentID string, limit int) ([]MetricBucket, error) {
	if limit <= 0 {
		limit = 5000
	}
	rows, err := database.pool.Query(ctx, `
		SELECT metric_name, bucket_start, bucket_end, min, max, avg, p95
		FROM segment_metrics WHERE segment_id = $1
		ORDER BY bucket_start ASC
		LIMIT $2`, segmentID, limit)
	if err != nil {
		return nil, fmt.Errorf("list segment metrics: %w", err)
	}
	defer rows.Close()
	var buckets []MetricBucket
	for rows.Next() {
		var bucket MetricBucket
		if err := rows.Scan(&bucket.MetricName, &bucket.BucketStart, &bucket.BucketEnd,
			&bucket.Min, &bucket.Max, &bucket.Avg, &bucket.P95); err != nil {
			return nil, fmt.Errorf("scan segment metric: %w", err)
		}
		buckets = append(buckets, bucket)
	}
	return buckets, rows.Err()
}

func (database *Postgres) ListSegmentTraces(ctx context.Context, segmentID string) ([]model.TraceSummary, error) {
	rows, err := database.pool.Query(ctx, `
		SELECT trace_id, start_time, duration_ms, root_service, root_operation,
		       tenant_name, model_name, simulator_instance_name, error_span_count, attributes
		FROM trace_index WHERE segment_id = $1
		ORDER BY start_time ASC`, segmentID)
	if err != nil {
		return nil, fmt.Errorf("list segment traces: %w", err)
	}
	defer rows.Close()
	var traces []model.TraceSummary
	for rows.Next() {
		var trace model.TraceSummary
		var tenant, modelName, instanceName string
		if err := rows.Scan(&trace.TraceID, &trace.StartTime, &trace.DurationMs,
			&trace.RootService, &trace.RootOperation, &tenant, &modelName, &instanceName,
			&trace.ErrorSpanCount, &trace.Entities); err != nil {
			return nil, fmt.Errorf("scan segment trace: %w", err)
		}
		trace.Entities = map[string]string{
			"tenant": tenant, "model": modelName, "simulatorInstance": instanceName,
		}
		traces = append(traces, trace)
	}
	return traces, rows.Err()
}

// scanSegment 扫描 segments 表的一行；参数兼容 pgx.Row 与 pgx.Rows。
type segmentScanner interface {
	Scan(dest ...any) error
}

func scanSegment(row segmentScanner) (*SegmentRecord, error) {
	var record SegmentRecord
	var startSnapshot, endSnapshot, summary []byte
	if err := row.Scan(
		&record.SegmentID, &record.Tenant, &record.Name, &record.Status, &record.Reason,
		&record.ConfigSnapshot, &startSnapshot, &endSnapshot, &summary,
		&record.StartedAt, &record.EndedAt, &record.CreatedAt, &record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	record.StartSnapshot = json.RawMessage(startSnapshot)
	record.EndSnapshot = json.RawMessage(endSnapshot)
	record.Summary = json.RawMessage(summary)
	return &record, nil
}
