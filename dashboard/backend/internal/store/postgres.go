package store

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Postgres struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func OpenPostgres(ctx context.Context, cfg config.DatabaseConfig, logger *slog.Logger) (*Postgres, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	poolConfig.MaxConns = cfg.MaxConnections
	poolConfig.MinConns = cfg.MinConnections
	poolConfig.MaxConnLifetime = cfg.MaxConnectionAge
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "hello-k8s-ai-dashboard-backend"

	connectContext, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(connectContext, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(connectContext); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return &Postgres{pool: pool, logger: logger}, nil
}

func (database *Postgres) Migrate(ctx context.Context) error {
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list embedded migrations: %w", err)
	}
	sort.Strings(files)

	tx, err := database.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('hello-k8s-ai-dashboard-migrations'))`); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
		)`); err != nil {
		return fmt.Errorf("ensure migration table: %w", err)
	}

	for _, filename := range files {
		var applied bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, filename).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}
		if applied {
			continue
		}
		contents, err := migrations.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}
		if _, err := tx.Exec(ctx, string(contents)); err != nil {
			return fmt.Errorf("apply migration %s: %w", filename, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, filename); err != nil {
			return fmt.Errorf("record migration %s: %w", filename, err)
		}
		database.logger.Info("Applied PostgreSQL migration", "version", filename)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func (database *Postgres) Health(ctx context.Context) error {
	if database == nil || database.pool == nil {
		return ErrUnavailable
	}
	return database.pool.Ping(ctx)
}

func (database *Postgres) RecordResourceChange(ctx context.Context, change model.ResourceChange) error {
	payload := change.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	_, err := database.pool.Exec(ctx, `
		INSERT INTO resource_events (
			event_id, occurred_at, operation, api_version, kind, namespace,
			name, uid, resource_version, generation, payload
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (event_id) DO NOTHING`,
		change.EventID, change.OccurredAt, change.Operation, change.Ref.APIVersion,
		change.Ref.Kind, change.Ref.Namespace, change.Ref.Name, change.Ref.UID,
		change.ResourceVersion, change.Generation, payload,
	)
	if err != nil {
		return fmt.Errorf("record resource change: %w", err)
	}
	return nil
}

func (database *Postgres) SaveSnapshot(ctx context.Context, snapshot SnapshotRecord) error {
	if snapshot.ID == "" {
		snapshot.ID = newID("snapshot")
	}
	if snapshot.CapturedAt.IsZero() {
		snapshot.CapturedAt = time.Now().UTC()
	}
	if snapshot.LogicalTime.IsZero() {
		snapshot.LogicalTime = snapshot.CapturedAt
	}
	_, err := database.pool.Exec(ctx, `
		INSERT INTO resource_snapshots (
			snapshot_id, captured_at, logical_time, source_versions, payload
		) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (snapshot_id) DO NOTHING`,
		snapshot.ID, snapshot.CapturedAt, snapshot.LogicalTime,
		snapshot.SourceVersions, snapshot.Payload,
	)
	if err != nil {
		return fmt.Errorf("save resource snapshot: %w", err)
	}
	return nil
}

func (database *Postgres) SnapshotAt(ctx context.Context, at time.Time) (*model.StoredSnapshot, error) {
	row := database.pool.QueryRow(ctx, `
		SELECT snapshot_id, captured_at, logical_time, payload
		FROM resource_snapshots
		WHERE captured_at <= $1
		ORDER BY captured_at DESC
		LIMIT 1`, at)
	var snapshot model.StoredSnapshot
	if err := row.Scan(&snapshot.ID, &snapshot.CapturedAt, &snapshot.LogicalTime, &snapshot.Payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("read resource snapshot: %w", err)
	}
	return &snapshot, nil
}

func (database *Postgres) ListTimeline(ctx context.Context, limit int, before *time.Time) ([]model.TimelineItem, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	cutoff := time.Now().UTC().Add(time.Second)
	if before != nil {
		cutoff = before.UTC()
	}
	rows, err := database.pool.Query(ctx, `
		SELECT timeline_id, occurred_at, operation, kind, namespace, name
		FROM (
			SELECT event_id AS timeline_id, occurred_at, operation, kind,
			       namespace, name, sequence
			FROM resource_events
			WHERE occurred_at < $1
			UNION ALL
			SELECT snapshot_id AS timeline_id, captured_at AS occurred_at,
			       'capture' AS operation, 'Snapshot' AS kind,
			       '' AS namespace, snapshot_id AS name, 0::bigint AS sequence
			FROM resource_snapshots
			WHERE captured_at < $1
		) AS timeline
		ORDER BY occurred_at DESC, sequence DESC
		LIMIT $2`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list timeline: %w", err)
	}
	defer rows.Close()

	items := make([]model.TimelineItem, 0, limit)
	for rows.Next() {
		var eventID, operation, kind, namespace, name string
		var occurredAt time.Time
		if err := rows.Scan(&eventID, &occurredAt, &operation, &kind, &namespace, &name); err != nil {
			return nil, fmt.Errorf("scan timeline item: %w", err)
		}
		items = append(items, timelineItem(eventID, occurredAt, operation, kind, namespace, name))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate timeline: %w", err)
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return items, nil
}

func (database *Postgres) RecordAudit(ctx context.Context, record model.AuditRecord) error {
	details := record.Details
	if len(details) == 0 {
		details = json.RawMessage(`{}`)
	}
	_, err := database.pool.Exec(ctx, `
		INSERT INTO audit_log (
			operation_id, occurred_at, actor, action, api_version, kind,
			namespace, name, uid, outcome, request_id, details
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		record.OperationID, record.OccurredAt, record.Actor, record.Action,
		record.Ref.APIVersion, record.Ref.Kind, record.Ref.Namespace, record.Ref.Name,
		record.Ref.UID, record.Outcome, record.RequestID, details,
	)
	if err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}
	return nil
}

func (database *Postgres) ReserveIdempotency(
	ctx context.Context,
	key string,
	requestHash string,
	expiresAt time.Time,
) (*IdempotencyRecord, bool, error) {
	tx, err := database.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin idempotency reservation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	record, err := scanIdempotency(tx.QueryRow(ctx, `
		INSERT INTO command_idempotency (
			idempotency_key, request_hash, state, expires_at
		) VALUES ($1, $2, 'pending', $3)
		ON CONFLICT (idempotency_key) DO UPDATE SET
			request_hash = EXCLUDED.request_hash,
			state = 'pending',
			status_code = NULL,
			response = NULL,
			created_at = clock_timestamp(),
			completed_at = NULL,
			expires_at = EXCLUDED.expires_at
		WHERE command_idempotency.expires_at <= clock_timestamp()
		RETURNING idempotency_key, request_hash, state,
			COALESCE(status_code, 0), response, created_at, expires_at`,
		key, requestHash, expiresAt,
	))
	owned := true
	if errors.Is(err, pgx.ErrNoRows) {
		owned = false
		record, err = scanIdempotency(tx.QueryRow(ctx, `
			SELECT idempotency_key, request_hash, state,
			       COALESCE(status_code, 0), response, created_at, expires_at
			FROM command_idempotency
			WHERE idempotency_key = $1`, key))
	}
	if err != nil {
		return nil, false, fmt.Errorf("reserve idempotency key: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit idempotency reservation: %w", err)
	}
	return record, owned, nil
}

func (database *Postgres) CompleteIdempotency(
	ctx context.Context,
	key string,
	requestHash string,
	statusCode int,
	response json.RawMessage,
) error {
	if !json.Valid(response) {
		return errors.New("idempotency response must be valid JSON")
	}
	result, err := database.pool.Exec(ctx, `
		UPDATE command_idempotency
		SET state = 'completed', status_code = $3, response = $4,
		    completed_at = clock_timestamp()
		WHERE idempotency_key = $1 AND request_hash = $2 AND state = 'pending'`,
		key, requestHash, statusCode, response,
	)
	if err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	if result.RowsAffected() != 1 {
		return errors.New("idempotency reservation no longer belongs to this request")
	}
	return nil
}

func (database *Postgres) ReleaseIdempotency(ctx context.Context, key string, requestHash string) error {
	_, err := database.pool.Exec(ctx, `
		DELETE FROM command_idempotency
		WHERE idempotency_key = $1 AND request_hash = $2 AND state = 'pending'`,
		key, requestHash,
	)
	if err != nil {
		return fmt.Errorf("release idempotency key: %w", err)
	}
	return nil
}

func (database *Postgres) IndexTraces(ctx context.Context, traces []model.TraceSummary) error {
	if len(traces) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, trace := range traces {
		batch.Queue(`
			INSERT INTO trace_index (
				trace_id, start_time, duration_ms, root_service, root_operation,
				tenant_name, model_name, simulator_instance_name, error_span_count, attributes
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (trace_id) DO UPDATE SET
				start_time=EXCLUDED.start_time,
				duration_ms=EXCLUDED.duration_ms,
				root_service=EXCLUDED.root_service,
				root_operation=EXCLUDED.root_operation,
				tenant_name=EXCLUDED.tenant_name,
				model_name=EXCLUDED.model_name,
				simulator_instance_name=EXCLUDED.simulator_instance_name,
				error_span_count=EXCLUDED.error_span_count,
				attributes=EXCLUDED.attributes,
				indexed_at=clock_timestamp()`,
			trace.TraceID, trace.StartTime, trace.DurationMs, trace.RootService,
			trace.RootOperation, trace.Entities["tenant"], trace.Entities["model"],
			trace.Entities["simulatorInstance"], trace.ErrorSpanCount, trace.Entities,
		)
	}
	results := database.pool.SendBatch(ctx, batch)
	for range traces {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("index trace: %w", err)
		}
	}
	return results.Close()
}

func (database *Postgres) Prune(ctx context.Context, before time.Time) error {
	tx, err := database.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin history prune: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, statement := range []string{
		`DELETE FROM resource_events WHERE occurred_at < $1`,
		`DELETE FROM resource_snapshots WHERE captured_at < $1`,
		`DELETE FROM trace_index WHERE start_time < $1`,
	} {
		if _, err := tx.Exec(ctx, statement, before); err != nil {
			return fmt.Errorf("prune dashboard history: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM command_idempotency WHERE expires_at < clock_timestamp()`); err != nil {
		return fmt.Errorf("prune expired command idempotency records: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit history prune: %w", err)
	}
	return nil
}

func scanIdempotency(row pgx.Row) (*IdempotencyRecord, error) {
	var record IdempotencyRecord
	if err := row.Scan(
		&record.Key,
		&record.RequestHash,
		&record.State,
		&record.StatusCode,
		&record.Response,
		&record.CreatedAt,
		&record.ExpiresAt,
	); err != nil {
		return nil, err
	}
	return &record, nil
}

func (database *Postgres) Close() {
	if database != nil && database.pool != nil {
		database.pool.Close()
	}
}

func (database *Postgres) Available() bool {
	return database != nil && database.pool != nil
}

func timelineItem(eventID string, occurredAt time.Time, operation, kind, namespace, name string) model.TimelineItem {
	if kind == "Snapshot" {
		return model.TimelineItem{
			ID: eventID, Timestamp: occurredAt.UTC(), Weight: 1,
			Type: "config", Trigger: "time", Domain: "runtime", Severity: "normal",
			Title:   "Kubernetes 状态切面",
			Summary: "Informer 聚合读模型已持久化到 PostgreSQL。",
			Source:  "postgresql/snapshot",
			Impact:  map[string]int{"tenants": 0, "nodes": 0, "models": 0, "changes": 0},
			Tags:    []string{"PostgreSQL", "Snapshot"},
		}
	}
	domain := "runtime"
	weight := 4
	if isConfigurationKind(kind) {
		domain = "configuration"
		weight = 3
	} else if kind == "Node" || kind == "WorkerNode" {
		domain = "capacity"
		weight = 5
	} else if kind == "Pod" || kind == "Deployment" || kind == "SimulatorInstance" || kind == "Orchestrator" {
		domain = "scheduler"
		weight = 6
	}
	severity := "normal"
	if operation == "delete" {
		severity = "attention"
		weight++
	}
	qualifiedName := name
	if namespace != "" {
		qualifiedName = namespace + "/" + name
	}
	return model.TimelineItem{
		ID:        eventID,
		Timestamp: occurredAt.UTC(),
		Weight:    weight,
		Type:      map[bool]string{true: "config", false: "event"}[domain == "configuration"],
		Trigger:   "event",
		Domain:    domain,
		Severity:  severity,
		Title:     fmt.Sprintf("%s %s", kind, operationLabel(operation)),
		Summary:   fmt.Sprintf("Kubernetes informer observed %s for %s %s.", operation, kind, qualifiedName),
		Source:    "kubernetes/informer",
		Impact:    impactForKind(kind),
		Tags:      []string{"Kubernetes", kind, operation},
	}
}

func operationLabel(operation string) string {
	switch strings.ToLower(operation) {
	case "add":
		return "已创建"
	case "delete":
		return "已删除"
	default:
		return "已更新"
	}
}

func isConfigurationKind(kind string) bool {
	switch kind {
	case "Model", "Tenant", "TenantModelPolicy", "TenantNodePolicy", "ModelNodePolicy":
		return true
	default:
		return false
	}
}

func impactForKind(kind string) map[string]int {
	impact := map[string]int{"tenants": 0, "nodes": 0, "models": 0, "changes": 1}
	switch kind {
	case "Tenant", "TenantPerformance", "TenantRuntime":
		impact["tenants"] = 1
	case "Node", "WorkerNode":
		impact["nodes"] = 1
	case "Model":
		impact["models"] = 1
	case "SimulatorInstance", "Orchestrator":
		impact["tenants"] = 1
		impact["models"] = 1
	}
	return impact
}

func newID(prefix string) string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return prefix + "-" + hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
