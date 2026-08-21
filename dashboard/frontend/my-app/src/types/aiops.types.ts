import type { ApiEnvelope } from '@/types/api.types'

/**
 * AIOps 前端契约（总纲 #92 / 前端面板 #98）。
 * 字段与后端 dashboard/backend/internal/model/aiops.go + 迁移 005_aiops.sql 对齐，
 * 作为前后端联调的评审点；后端 M2/M3 未就绪的类型先落契约，联调后再接真实 API。
 */

/** 分析状态机：pending → running(L1) → aggregating(L2) → completed/failed。 */
export type AIOpsAnalysisStatus =
    | 'pending'
    | 'running'
    | 'aggregating'
    | 'completed'
    | 'failed'

/** L1 实体分类：前端以颜色区分（优质/可疑/问题）。 */
export type AIOpsClassification = 'healthy' | 'suspect' | 'problem'

/** 实体 kind（与后端 L1 提示词输出对齐）。 */
export type AIOpsEntityKind = 'Pod' | 'Node' | 'Tenant'

/** L2 分维度结构（goal 目标达成 / stability 稳定性 / efficiency 效率 / anomaly 异常）。 */
export interface AIOpsScores {
    goal: number
    stability: number
    efficiency: number
    anomaly: number
    overall: number
    verdict: string
    reason: string
}

/** aiops_analyses 一行：一次切面的 L1/L2 分析主记录。 */
export interface AIOpsAnalysis {
    analysisId: string
    segmentId: string
    status: AIOpsAnalysisStatus
    l1Total: number
    l1Done: number
    scores?: AIOpsScores
    summary?: unknown
    error?: string
    createdAt: string
    updatedAt: string
}

/** aiops_entity_summaries 一行：L1 单实体总结。 */
export interface AIOpsEntitySummary {
    summaryId: string
    analysisId: string
    entityKind: AIOpsEntityKind
    entityName: string
    classification: AIOpsClassification
    phenomenon: string
    issueFlag: boolean
    conclusion: string
    createdAt: string
}

/** 分析详情：主记录 + L1 实体总结列表。 */
export interface AIOpsAnalysisDetail {
    analysis: AIOpsAnalysis
    entities: AIOpsEntitySummary[]
}

/** 气泡 Agent 分级（ClusterBubbleField 外圈着色；后端 AI 结论接入前不渲染）。 */
export type AgentGrade = 'normal' | 'odd' | 'problematic'

export interface AgentVerdict {
    grade?: AgentGrade
    score?: number
    summary?: string
    updatedAt?: string
}

/** 契约先行：L3/L4 时间聚合（M3 启用）。 */
export type AIOpsWindowLevel = 'L3' | 'L4'

export interface AIOpsWindowSummary {
    windowId: string
    level: AIOpsWindowLevel
    windowStart: string
    windowEnd: string
    scores?: AIOpsScores
    summary?: unknown
    createdAt: string
}

/** 契约先行：持续低分警戒（M3 启用）。 */
export type AIOpsAlertSeverity = 'info' | 'warning' | 'critical'

export interface AIOpsAlert {
    alertId: string
    rule: string
    severity: AIOpsAlertSeverity
    triggeredAt: string
    analysisId?: string
    interpretation?: unknown
    ackedAt?: string
}

/** 契约先行：意图执行（M2 启用）。 */
export type AIOpsCommandStatus =
    | 'parsed'
    | 'confirmed'
    | 'gate'
    | 'executing'
    | 'verified'
    | 'done'
    | 'rejected'
    | 'failed'

export interface AIOpsCommand {
    commandId: string
    rawInput: string
    parsed: unknown
    status: AIOpsCommandStatus
    steps: unknown[]
    errorText?: string
    createdAt: string
    updatedAt: string
}

export type AIOpsAnalysesEnvelope = ApiEnvelope<AIOpsAnalysis[]>
export type AIOpsAnalysisDetailEnvelope = ApiEnvelope<AIOpsAnalysisDetail>
export type AIOpsAlertsEnvelope = ApiEnvelope<AIOpsAlert[]>
export type AIOpsCommandEnvelope = ApiEnvelope<AIOpsCommand>
export type AIOpsWindowsEnvelope = ApiEnvelope<AIOpsWindowSummary[]>

/** 同步对话 SSE 事件（#110 阶段二，AG-UI 轻量子集）：lifecycle / tool / text。 */
export interface AIOpsChatEvent {
    type: 'lifecycle' | 'tool' | 'text'
    phase?: 'start' | 'end'
    name?: string
    delta?: string
    sessionId?: string
    error?: string
    durationMs?: number
}
