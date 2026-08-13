import { useQuery } from '@tanstack/react-query'
import { fetchOverview, fetchTrace } from '@/api/endpoints/traceApi'
import type { OverviewQuery } from '@/types/trace.types'

export const traceQueryKeys = {
    all: ['trace'] as const,
    overview: (query: OverviewQuery) => [
        ...traceQueryKeys.all,
        'overview',
        query.mode,
        query.mode === 'historical' ? query.snapshotId : 'latest',
        query.tenantId ?? null,
        query.modelId ?? null,
        query.instanceId ?? null,
    ] as const,
    detail: (traceId: string | null) => [...traceQueryKeys.all, 'detail', traceId] as const,
}

export function useTraceDetail(traceId: string | null) {
    return useQuery({
        queryKey: traceQueryKeys.detail(traceId),
        queryFn: () => fetchTrace(traceId!),
        enabled: Boolean(traceId),
        staleTime: Number.POSITIVE_INFINITY,
        retry: 1,
    })
}

export function useOverview(query: OverviewQuery) {
    return useQuery({
        queryKey: traceQueryKeys.overview(query),
        queryFn: () => fetchOverview(query),
        staleTime: query.mode === 'latest' ? 8_000 : Number.POSITIVE_INFINITY,
        refetchInterval: query.mode === 'latest' ? 15_000 : false,
        retry: 1,
    })
}
