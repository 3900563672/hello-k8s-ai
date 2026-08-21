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
    /** 失败重试计数（#110 阶段一：claim 时 +1，达上限转 failed）。 */
    attempts?: number
    /** 任务类型（segment=切面 L1/L2 分析，预留后续类型）。 */
    kind?: string
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

/** AIOps 意图执行硬限制（GET /aiops/limits，解析校验与前端提示共用同一事实源）。 */
export interface AIOpsLimits {
    maxTrafficQPS: number
    maxSimulationRate: number
    trafficShapes: string[]
    defaultTidalPeriodMinutes: number
    defaultPeakQPS: number
    defaultRate: number
    trafficRequiresTenant: boolean
    unlimitedDuration: boolean
    supportsStop: boolean
}

/** 生效参数记录（#134）：请求值→生效值+原因（clamped-to-max/defaulted/ok）。 */
export interface AIOpsAppliedValue {
    field: string
    requested?: number
    effective: number
    reason: 'ok' | 'clamped-to-max' | 'defaulted'
}

/** 波形采样点（AI 描绘，#134）：x 为模拟秒，y 为 QPS。 */
export interface AIOpsTrafficPoint {
    x: number
    y: number
}

/** 命令的生效参数与 AI 描绘波形（后端动态计算，不落库，#134）。 */
export interface AIOpsTrafficApplied {
    values: AIOpsAppliedValue[]
    curve: AIOpsTrafficPoint[]
    wallClockSeconds: number
}

/** 日配额用量（GET /aiops/quota，#134：被拒前先知道还剩多少）。 */
export interface AIOpsQuota {
    enabled: boolean
    callsUsed: number
    callsMax: number
    tokensUsed: number
    tokensMax: number
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
    | 'stopped'

export interface AIOpsCommand {
    commandId: string
    rawInput: string
    parsed: unknown
    status: AIOpsCommandStatus
    steps: unknown[]
    errorText?: string
    createdAt: string
    updatedAt: string
    /** 生效参数与 AI 描绘波形（#134，动态计算）。 */
    applied?: AIOpsTrafficApplied
}

export type AIOpsAnalysesEnvelope = ApiEnvelope<AIOpsAnalysis[]>
export type AIOpsAnalysisDetailEnvelope = ApiEnvelope<AIOpsAnalysisDetail>
export type AIOpsAlertsEnvelope = ApiEnvelope<AIOpsAlert[]>
export type AIOpsCommandEnvelope = ApiEnvelope<AIOpsCommand>
export type AIOpsQuotaEnvelope = ApiEnvelope<AIOpsQuota>
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

/** aiops_chat_messages 一行：同步对话的问答对（#112 阶段 D 读侧）。 */
export interface AIOpsChatMessage {
    messageId: string
    sessionId: string
    role: 'user' | 'assistant'
    content: string
    windowIds?: string[]
    alertIds?: string[]
    commandIds?: string[]
    createdAt: string
}

export type AIOpsChatMessagesEnvelope = ApiEnvelope<AIOpsChatMessage[]>

/** 异步任务状态（#110 阶段一）：DB 即队列，pending→running→done/failed。 */
export type AIOpsJobStatus = 'pending' | 'running' | 'done' | 'failed'

/** 异步任务（#110 阶段一）：任务级状态 / 重试次数 / 失败原因。 */
export interface AIOpsJob {
    jobId: string
    segmentId: string
    kind: string
    status: AIOpsJobStatus
    attempts: number
    maxAttempts: number
    lastError?: string
    createdAt: string
    startedAt?: string
    finishedAt?: string
    updatedAt: string
}

export type AIOpsJobsEnvelope = ApiEnvelope<AIOpsJob[]>

/** LLM 配置掩码状态（#110 阶段四）：key 只显示是否配置，不回显明文。 */
export interface AIOpsSettings {
    configured: boolean
    model: string
    baseUrl: string
    keyConfigured: boolean
    enabled: boolean
}

export type AIOpsSettingsEnvelope = ApiEnvelope<AIOpsSettings>
