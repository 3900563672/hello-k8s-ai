import { API_BASE_URL, ApiRequestError, apiRequest } from '@/api/client'
import type { ApiProblemEnvelope } from '@/types/api.types'
import type {
    AIOpsAnalysesEnvelope,
    AIOpsAnalysisDetailEnvelope,
    AIOpsAlertsEnvelope,
    AIOpsChatEvent,
    AIOpsCommandEnvelope,
    AIOpsJobStatus,
    AIOpsJobsEnvelope,
    AIOpsSettingsEnvelope,
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

/** 同步对话回调（#110 阶段二）。 */
export interface AIOpsChatHandlers {
    onLifecycle?: (phase: 'start' | 'end', error?: string, durationMs?: number) => void
    onTool?: (name: string, phase: 'start' | 'end') => void
    onText?: (delta: string) => void
}

/**
 * 同步对话（SSE 流式）：POST /aiops/chat，逐事件解析并分发。
 * 事件格式为 `data: {json}\n\n`；流结束后返回。
 * 非 2xx 或网络错误会抛出 ApiRequestError（404 = AIOps 未启用）。
 */
export async function streamAIOpsChat(
    message: string,
    sessionId: string,
    handlers: AIOpsChatHandlers,
): Promise<void> {
    const response = await fetch(`${API_BASE_URL}/aiops/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
        body: JSON.stringify({ message, sessionId }),
    })
    if (!response.ok) {
        const body = (await response.json().catch(() => null)) as ApiProblemEnvelope | null
        const problem = body?.error ?? null
        throw new ApiRequestError(
            response.status,
            problem?.message ?? `请求失败（${response.status}）`,
            problem,
        )
    }
    const reader = response.body?.getReader()
    if (!reader) {
        throw new ApiRequestError(0, '浏览器不支持流式响应')
    }
    const decoder = new TextDecoder()
    let buffer = ''
    for (;;) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        let newlineIndex: number
        while ((newlineIndex = buffer.indexOf('\n')) >= 0) {
            const line = buffer.slice(0, newlineIndex).trim()
            buffer = buffer.slice(newlineIndex + 1)
            if (!line.startsWith('data:')) continue
            const data = line.slice(5).trim()
            if (!data || data === '[DONE]') continue
            let event: AIOpsChatEvent
            try {
                event = JSON.parse(data) as AIOpsChatEvent
            } catch {
                continue
            }
            if (event.type === 'lifecycle') {
                handlers.onLifecycle?.(event.phase === 'start' ? 'start' : 'end', event.error, event.durationMs)
            } else if (event.type === 'tool') {
                handlers.onTool?.(event.name ?? '', event.phase === 'start' ? 'start' : 'end')
            } else if (event.type === 'text' && event.delta) {
                handlers.onText?.(event.delta)
            }
        }
    }
}

/** 异步任务列表（#110 阶段一）：status 可过滤，默认倒序。 */
export function fetchAIOpsJobs(status?: AIOpsJobStatus, limit = 20): Promise<AIOpsJobsEnvelope> {
    const params = new URLSearchParams()
    if (status) params.set('status', status)
    params.set('limit', String(limit))
    return apiRequest<AIOpsJobsEnvelope>(`/aiops/jobs?${params.toString()}`)
}

/** 读取 LLM 配置掩码状态（key 不回显）。 */
export function fetchAIOpsSettings(): Promise<AIOpsSettingsEnvelope> {
    return apiRequest<AIOpsSettingsEnvelope>('/aiops/settings')
}

/** 面板写入 LLM 配置：key 仅存服务端内存，返回掩码状态。 */
export function updateAIOpsSettings(payload: {
    apiKey?: string
    model?: string
    baseUrl?: string
}): Promise<AIOpsSettingsEnvelope> {
    return apiRequest<AIOpsSettingsEnvelope>('/aiops/settings', {
        method: 'POST',
        body: JSON.stringify(payload),
    })
}
