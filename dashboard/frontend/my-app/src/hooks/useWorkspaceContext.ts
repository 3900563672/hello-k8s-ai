import { useEffect, useMemo } from 'react'
import {
    canRunTest,
    onlineWorkerCount,
    useControlPlaneStore,
} from '@/stores/controlPlaneSlice'
import {
    selectSelectedSnapshot,
    useReplayTimeContext,
    useTimeStore,
} from '@/stores/timeSlice'

/**
 * 所有页面消费同一份工作区语义：时间、集群与执行模式不再各自为政。
 */
export function useWorkspaceContext() {
    const replay = useReplayTimeContext()
    const selectedSnapshot = useTimeStore(selectSelectedSnapshot)
    const cluster = useControlPlaneStore((state) => state.cluster)
    const executionMode = useControlPlaneStore((state) => state.executionMode)
    const executionPhase = useControlPlaneStore((state) => state.executionPhase)

    return useMemo(() => {
        const onlineWorkers = onlineWorkerCount(cluster)
        const isHistorical = replay.mode === 'historical'

        return {
            ...replay,
            selectedSnapshot,
            cluster,
            onlineWorkers,
            totalWorkers: cluster.workers.length,
            executionMode,
            executionPhase,
            isHistorical,
            isWritable: !isHistorical,
            canTest: !isHistorical && canRunTest(cluster),
        }
    }, [cluster, executionMode, executionPhase, replay, selectedSnapshot])
}

/**
 * 历史回放或集群不可用时，测试执行必须自动退回安全的应用模式。
 */
export function useWorkspaceCoordinator() {
    const timeMode = useTimeStore((state) => state.mode)
    const cluster = useControlPlaneStore((state) => state.cluster)
    const executionMode = useControlPlaneStore((state) => state.executionMode)
    const forceApplyMode = useControlPlaneStore((state) => state.forceApplyMode)

    useEffect(() => {
        if (
            executionMode === 'test' &&
            (timeMode === 'historical' || !canRunTest(cluster))
        ) {
            forceApplyMode()
        }
    }, [cluster, executionMode, forceApplyMode, timeMode])
}
