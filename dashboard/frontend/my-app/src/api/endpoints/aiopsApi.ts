import { apiRequest } from '@/api/client'
import type {
    AIOpsAnalysesEnvelope,
    AIOpsAnalysisDetailEnvelope,
    AIOpsAlertsEnvelope,
    AIOpsCommandEnvelope,
    AIOpsWindowLevel,
    AIOpsWindowsEnvelope,
} from '@/types/aiops.types'

/** AIOps 分析列表（status 可过滤，默认按时间倒序）。 */
export function fetchAIOpsAnalyses(status?: string, limit = 50): Promise<AIOpsAnalysesEnvelope> {
    const params = new URLSearchParams()
    if (status) params.set('status', status)
    params.set('limit', String(limit))
    return apiRequest<AIOpsAnalysesEnvelope>(`/aiops/analyses?${params.toString()}`)
}

/** 单条分析详情（主记录 + L1 实体总结）。 */
export function fetchAIOpsAnalysis(analysisId: string): Promise<AIOpsAnalysisDetailEnvelope> {
    return apiRequest<AIOpsAnalysisDetailEnvelope>(
        `/aiops/analyses/${encodeURIComponent(analysisId)}`,
    )
}

/** 按切面查询分析详情（后端支持 ?segmentId=）。 */
export function fetchAIOpsAnalysisBySegment(segmentId: string): Promise<AIOpsAnalysisDetailEnvelope> {
    return apiRequest<AIOpsAnalysisDetailEnvelope>(
        `/aiops/analyses?segmentId=${encodeURIComponent(segmentId)}`,
    )
}

/** 只读模板目录（LLM 与前端确认共用）。 */
export function fetchAIOpsTemplates(): Promise<unknown> {
    return apiRequest('/aiops/templates')
}

/** 一句话意图：解析并落库 parsed，返回命令（含 commandId 与解析结果）。 */
export function createAIOpsCommand(rawInput: string): Promise<AIOpsCommandEnvelope> {
    return apiRequest<AIOpsCommandEnvelope>('/aiops/commands', {
        method: 'POST',
        body: JSON.stringify({ rawInput }),
    })
}

/** 确认并执行意图：gate 校验 → 写流量/调倍速 → 创建并启动实验 → done。 */
export function confirmAIOpsCommand(commandId: string): Promise<AIOpsCommandEnvelope> {
    return apiRequest<AIOpsCommandEnvelope>(
        `/aiops/commands/${encodeURIComponent(commandId)}/confirm`,
        { method: 'POST' },
    )
}

/** 查询意图命令（含解析结果与执行 steps）。 */
export function fetchAIOpsCommand(commandId: string): Promise<AIOpsCommandEnvelope> {
    return apiRequest<AIOpsCommandEnvelope>(
        `/aiops/commands/${encodeURIComponent(commandId)}`,
    )
}

/** L3/L4 窗口/日总结列表。 */
export function fetchAIOpsWindows(level: AIOpsWindowLevel = 'L3', limit = 20): Promise<AIOpsWindowsEnvelope> {
    return apiRequest<AIOpsWindowsEnvelope>(
        `/aiops/windows?level=${level}&limit=${limit}`,
    )
}

/** 警戒列表。 */
export function fetchAIOpsAlerts(limit = 20): Promise<AIOpsAlertsEnvelope> {
    return apiRequest<AIOpsAlertsEnvelope>(`/aiops/alerts?limit=${limit}`)
}
