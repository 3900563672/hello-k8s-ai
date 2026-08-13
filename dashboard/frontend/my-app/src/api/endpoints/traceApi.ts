import { apiRequest } from '@/api/client'
import type { OverviewEnvelope, OverviewQuery, TraceDetailEnvelope } from '@/types/trace.types'

export function fetchOverview(query: OverviewQuery): Promise<OverviewEnvelope> {
    const parameters = new URLSearchParams()
    if (query.mode === 'historical') parameters.set('at', query.effectiveAt)
    if (query.tenantId) parameters.set('tenant', query.tenantId)
    if (query.modelId) parameters.set('model', query.modelId)
    if (query.instanceId) parameters.set('instance', query.instanceId)
    const suffix = parameters.size > 0 ? `?${parameters.toString()}` : ''
    return apiRequest<OverviewEnvelope>(`/overview${suffix}`)
}

export function fetchTrace(traceId: string): Promise<TraceDetailEnvelope> {
    return apiRequest<TraceDetailEnvelope>(`/traces/${encodeURIComponent(traceId)}`)
}
