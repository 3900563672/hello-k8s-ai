import { apiRequest } from '@/api/client'
import type {
    AIOpsAnalysesEnvelope,
    AIOpsAnalysisDetailEnvelope,
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
