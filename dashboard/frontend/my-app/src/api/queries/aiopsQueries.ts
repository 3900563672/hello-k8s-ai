import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
    confirmAIOpsCommand,
    createAIOpsCommand,
    fetchAIOpsAlerts,
    fetchAIOpsJobs,
    fetchAIOpsLimits,
    fetchAIOpsWindows,
} from '@/api/endpoints/aiopsApi'
import type { AIOpsWindowLevel } from '@/types/aiops.types'

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
    limits: ['aiops', 'limits'] as const,
}

/** 意图执行硬限制：确认面板提示条展示（失败静默，不影响起实验主流程）。 */
export function useAIOpsLimits() {
    return useQuery({
        queryKey: aiopsQueryKeys.limits,
        queryFn: () => fetchAIOpsLimits(),
        staleTime: 5 * 60_000,
        retry: 0,
    })
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

/** L3/L4 窗口/日总结：30 秒轮询（定时产出，无需高频）。 */
export function useAIOpsWindows(level: AIOpsWindowLevel = 'L3') {
    return useQuery({
        queryKey: [...aiopsQueryKeys.all, 'windows', level] as const,
        queryFn: () => fetchAIOpsWindows(level),
        refetchInterval: 30_000,
        staleTime: 20_000,
        retry: 0,
    })
}

/** 警戒列表：30 秒轮询。 */
export function useAIOpsJobs() {
    return useQuery({
        queryKey: ['aiops', 'jobs'],
        queryFn: () => fetchAIOpsJobs(),
        refetchInterval: 10_000,
        staleTime: 5_000,
    })
}

export function useAIOpsAlerts() {
    return useQuery({
        queryKey: [...aiopsQueryKeys.all, 'alerts'] as const,
        queryFn: () => fetchAIOpsAlerts(),
        refetchInterval: 30_000,
        staleTime: 20_000,
        retry: 0,
    })
}

/** 一句话意图：解析落库（mutation；返回命令含解析预览）。 */
export function useCreateAIOpsCommand() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (rawInput: string) => createAIOpsCommand(rawInput),
        onSuccess: () => {
            void queryClient.invalidateQueries({ queryKey: aiopsQueryKeys.all })
        },
    })
}

/** 确认执行意图：gate → 执行 → done/failed（mutation）。 */
export function useConfirmAIOpsCommand() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (commandId: string) => confirmAIOpsCommand(commandId),
        onSuccess: () => {
            void queryClient.invalidateQueries({ queryKey: aiopsQueryKeys.all })
        },
    })
}
