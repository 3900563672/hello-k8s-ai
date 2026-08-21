package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/jackc/pgx/v5"
)

// CreateAIOpsAnalysis 为切面创建 pending 分析记录；同切面重复入队幂等（segment_id 唯一）。
func (database *Postgres) CreateAIOpsAnalysis(ctx context.Context, analysis model.AIOpsAnalysis) error {
	_, err := database.pool.Exec(ctx, `
		INSERT INTO aiops_analyses (analysis_id, segment_id, status, l1_total, l1_done)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (segment_id) DO NOTHING`,
		analysis.AnalysisID, analysis.SegmentID, analysis.Status, analysis.L1Total, analysis.L1Done)
	if err != nil {
		return fmt.Errorf("insert aiops analysis: %w", err)
	}
	return nil
}

// ClaimAIOpsAnalysis 原子认领 pending 分析（pending→running），返回是否抢到。
func (database *Postgres) ClaimAIOpsAnalysis(ctx context.Context, analysisID string) (bool, error) {
	tag, err := database.pool.Exec(ctx, `
		UPDATE aiops_analyses SET status='running', updated_at=clock_timestamp()
		WHERE analysis_id=$1 AND status='pending'`, analysisID)
	if err != nil {
		return false, fmt.Errorf("claim aiops analysis: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// RequeueStaleAIOpsAnalyses 启动时回收崩溃遗留的 running/aggregating 分析，返回回收数量。
func (database *Postgres) RequeueStaleAIOpsAnalyses(ctx context.Context, cutoff time.Time) (int, error) {
	tag, err := database.pool.Exec(ctx, `
		UPDATE aiops_analyses SET status='pending', updated_at=clock_timestamp()
		WHERE status IN ('running', 'aggregating') AND updated_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("requeue stale aiops analyses: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// UpdateAIOpsAnalysisProgress 更新状态机与 L1 进度（running/aggregating 阶段通用）。
func (database *Postgres) UpdateAIOpsAnalysisProgress(ctx context.Context, analysisID, status string, l1Total, l1Done int, errorText string) error {
	_, err := database.pool.Exec(ctx, `
		UPDATE aiops_analyses
		SET status=$2, l1_total=$3, l1_done=$4, error_text=$5, updated_at=clock_timestamp()
		WHERE analysis_id=$1`,
		analysisID, status, l1Total, l1Done, errorText)
	if err != nil {
		return fmt.Errorf("update aiops analysis progress: %w", err)
	}
	return nil
}

// CompleteAIOpsAnalysis 写入 L2 分数与总结，状态置 completed。
func (database *Postgres) CompleteAIOpsAnalysis(ctx context.Context, analysisID string, scores, summary json.RawMessage) error {
	_, err := database.pool.Exec(ctx, `
		UPDATE aiops_analyses
		SET status='completed', scores=$2, summary=$3, error_text='', updated_at=clock_timestamp()
		WHERE analysis_id=$1`,
		analysisID, scores, summary)
	if err != nil {
		return fmt.Errorf("complete aiops analysis: %w", err)
	}
	return nil
}

// FailAIOpsAnalysis 标记分析失败（终态）。
func (database *Postgres) FailAIOpsAnalysis(ctx context.Context, analysisID, errorText string) error {
	_, err := database.pool.Exec(ctx, `
		UPDATE aiops_analyses SET status='failed', error_text=$2, updated_at=clock_timestamp()
		WHERE analysis_id=$1`, analysisID, errorText)
	if err != nil {
		return fmt.Errorf("fail aiops analysis: %w", err)
	}
	return nil
}

func scanAIOpsAnalysis(row pgx.Row) (*model.AIOpsAnalysis, error) {
	var analysis model.AIOpsAnalysis
	err := row.Scan(&analysis.AnalysisID, &analysis.SegmentID, &analysis.Status,
		&analysis.L1Total, &analysis.L1Done, &analysis.Scores, &analysis.Summary,
		&analysis.Error, &analysis.CreatedAt, &analysis.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &analysis, nil
}

func (database *Postgres) GetAIOpsAnalysis(ctx context.Context, analysisID string) (*model.AIOpsAnalysis, error) {
	analysis, err := scanAIOpsAnalysis(database.pool.QueryRow(ctx, `
		SELECT analysis_id, segment_id, status, l1_total, l1_done, scores, summary, error_text, created_at, updated_at
		FROM aiops_analyses WHERE analysis_id=$1`, analysisID))
	if err != nil {
		return nil, fmt.Errorf("get aiops analysis: %w", err)
	}
	return analysis, nil
}

func (database *Postgres) GetAIOpsAnalysisBySegment(ctx context.Context, segmentID string) (*model.AIOpsAnalysis, error) {
	analysis, err := scanAIOpsAnalysis(database.pool.QueryRow(ctx, `
		SELECT analysis_id, segment_id, status, l1_total, l1_done, scores, summary, error_text, created_at, updated_at
		FROM aiops_analyses WHERE segment_id=$1`, segmentID))
	if err != nil {
		return nil, fmt.Errorf("get aiops analysis by segment: %w", err)
	}
	return analysis, nil
}

func (database *Postgres) ListAIOpsAnalyses(ctx context.Context, limit int, status string) ([]model.AIOpsAnalysis, error) {
	rows, err := database.pool.Query(ctx, `
		SELECT analysis_id, segment_id, status, l1_total, l1_done, scores, summary, error_text, created_at, updated_at
		FROM aiops_analyses
		WHERE ($1 = '' OR status = $1)
		ORDER BY created_at DESC
		LIMIT $2`, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list aiops analyses: %w", err)
	}
	defer rows.Close()
	var analyses []model.AIOpsAnalysis
	for rows.Next() {
		var analysis model.AIOpsAnalysis
		if err := rows.Scan(&analysis.AnalysisID, &analysis.SegmentID, &analysis.Status,
			&analysis.L1Total, &analysis.L1Done, &analysis.Scores, &analysis.Summary,
			&analysis.Error, &analysis.CreatedAt, &analysis.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan aiops analysis: %w", err)
		}
		analyses = append(analyses, analysis)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate aiops analyses: %w", err)
	}
	return analyses, nil
}

// UpsertAIOpsEntitySummaries 批量写入 L1 实体总结；按 (analysis_id, entity_kind, entity_name) 幂等更新。
func (database *Postgres) UpsertAIOpsEntitySummaries(ctx context.Context, analysisID string, summaries []model.AIOpsEntitySummary) error {
	if len(summaries) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, summary := range summaries {
		batch.Queue(`
			INSERT INTO aiops_entity_summaries (summary_id, analysis_id, entity_kind, entity_name, classification, phenomenon, issue_flag, conclusion)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (analysis_id, entity_kind, entity_name) DO UPDATE SET
				classification = EXCLUDED.classification,
				phenomenon = EXCLUDED.phenomenon,
				issue_flag = EXCLUDED.issue_flag,
				conclusion = EXCLUDED.conclusion`,
			summary.SummaryID, analysisID, summary.EntityKind, summary.EntityName,
			summary.Classification, summary.Phenomenon, summary.IssueFlag, summary.Conclusion)
	}
	if err := database.pool.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("upsert aiops entity summaries: %w", err)
	}
	return nil
}

func (database *Postgres) ListAIOpsEntitySummaries(ctx context.Context, analysisID string) ([]model.AIOpsEntitySummary, error) {
	rows, err := database.pool.Query(ctx, `
		SELECT summary_id, analysis_id, entity_kind, entity_name, classification, phenomenon, issue_flag, conclusion, created_at
		FROM aiops_entity_summaries WHERE analysis_id=$1
		ORDER BY issue_flag DESC, created_at ASC`, analysisID)
	if err != nil {
		return nil, fmt.Errorf("list aiops entity summaries: %w", err)
	}
	defer rows.Close()
	var summaries []model.AIOpsEntitySummary
	for rows.Next() {
		var summary model.AIOpsEntitySummary
		if err := rows.Scan(&summary.SummaryID, &summary.AnalysisID, &summary.EntityKind,
			&summary.EntityName, &summary.Classification, &summary.Phenomenon,
			&summary.IssueFlag, &summary.Conclusion, &summary.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan aiops entity summary: %w", err)
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate aiops entity summaries: %w", err)
	}
	return summaries, nil
}
