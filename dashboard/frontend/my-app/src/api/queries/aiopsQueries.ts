import { useQuery } from '@tanstack/react-query'
import {
    fetchAIOpsAnalysisBySegment,
    fetchAIOpsAnalyses,
} from '@/api/endpoints/aiopsApi'
import type { AIOpsAnalysisStatus } from '@/types/aiops.types'

export const aiopsQueryKeys = {
    all: ['aiops'] as const,
    analyses: (status?: AIOpsAnalysisStatus) =>
        [...aiopsQueryKeys.all, 'analyses', status ?? null] as const,
    detail: (segmentId: string) => [...aiopsQueryKeys.all, 'detail', segmentId] as const,
}

/** 分析列表：15 秒轮询（状态机进行中时进度会推进）。 */
export function useAIOpsAnalyses(status?: AIOpsAnalysisStatus) {
    return useQuery({
        queryKey: aiopsQueryKeys.analyses(status),
        queryFn: () => fetchAIOpsAnalyses(status),
        refetchInterval: 15_000,
        staleTime: 10_000,
        retry: 0,
    })
}

/** 按切面查详情：选中切面后拉取，L1 进度变化靠手动刷新/轮询。 */
export function useAIOpsAnalysisBySegment(segmentId: string | null) {
    return useQuery({
        queryKey: aiopsQueryKeys.detail(segmentId ?? ''),
        queryFn: () => fetchAIOpsAnalysisBySegment(segmentId!),
        enabled: Boolean(segmentId),
        refetchInterval: (query) =>
            query.state.data?.data.analysis.status === 'completed' ||
            query.state.data?.data.analysis.status === 'failed'
                ? false
                : 10_000,
        staleTime: 10_000,
        retry: 0,
    })
}
