import { create } from 'zustand'
import { devtools } from 'zustand/middleware'
import {
    createInitialCluster,
    distributeConfiguration,
    fetchClusterSnapshot,
    updateSimulationRate,
} from '@/api/endpoints/controlPlaneApi'
import type {
    AsyncPhase,
    ClusterSnapshot,
    DistributionReceipt,
    ExecutionMode,
    ExecutionPhase,
} from '@/types/control-plane.types'

interface ControlPlaneState {
    cluster: ClusterSnapshot
    refreshPhase: AsyncPhase
    distributionPhase: AsyncPhase
    simulationRatePhase: AsyncPhase
    distributionReceipt: DistributionReceipt | null
    executionMode: ExecutionMode
    executionPhase: ExecutionPhase
    lastError: string | null
    refreshCluster: () => Promise<void>
    distributeConfig: () => Promise<DistributionReceipt | null>
    setSimulationRate: (rate: number) => Promise<boolean>
    clearDistributionFeedback: () => void
    setExecutionMode: (mode: ExecutionMode) => boolean
    forceApplyMode: () => void
}

const errorMessage = (error: unknown) =>
    error instanceof Error ? error.message : '控制平面操作失败'

export const onlineWorkerCount = (cluster: ClusterSnapshot) =>
    cluster.workers.filter((node) => node.ready && node.status === 'running').length

export const canRunTest = (cluster: ClusterSnapshot) =>
    cluster.simulationRunSupported &&
    cluster.connectionStatus === 'connected' &&
    onlineWorkerCount(cluster) > 0

export const useControlPlaneStore = create<ControlPlaneState>()(
    devtools(
        (set, get) => ({
            cluster: createInitialCluster(),
            refreshPhase: 'idle',
            distributionPhase: 'idle',
            simulationRatePhase: 'idle',
            distributionReceipt: null,
            executionMode: 'apply',
            executionPhase: 'standby',
            lastError: null,

            refreshCluster: async () => {
                const current = get().cluster
                if (get().refreshPhase === 'pending') return

                set(
                    {
                        cluster: { ...current, connectionStatus: 'connecting' },
                        refreshPhase: 'pending',
                        lastError: null,
                    },
                    false,
                    'control-plane/refresh-started',
                )

                try {
                    const cluster = await fetchClusterSnapshot(current)
                    set(
                        { cluster, refreshPhase: 'success' },
                        false,
                        'control-plane/refresh-succeeded',
                    )
                } catch (error) {
                    set(
                        {
                            cluster: { ...current, connectionStatus: 'disconnected' },
                            refreshPhase: 'error',
                            executionMode: 'apply',
                            executionPhase: 'error',
                            lastError: errorMessage(error),
                        },
                        false,
                        'control-plane/refresh-failed',
                    )
                }
            },

            distributeConfig: async () => {
                if (get().distributionPhase === 'pending') return null
                set(
                    {
                        distributionPhase: 'pending',
                        distributionReceipt: null,
                        lastError: null,
                    },
                    false,
                    'control-plane/distribution-started',
                )

                try {
                    const receipt = await distributeConfiguration(get().cluster)
                    set(
                        {
                            distributionPhase: 'success',
                            distributionReceipt: receipt,
                        },
                        false,
                        'control-plane/distribution-succeeded',
                    )
                    return receipt
                } catch (error) {
                    set(
                        {
                            distributionPhase: 'error',
                            lastError: errorMessage(error),
                        },
                        false,
                        'control-plane/distribution-failed',
                    )
                    return null
                }
            },

            setSimulationRate: async (rate) => {
                const state = get()
                if (
                    state.simulationRatePhase === 'pending' ||
                    !state.cluster.simulationRateSupported ||
                    state.cluster.connectionStatus !== 'connected' ||
                    (Boolean(state.cluster.clockResourceVersion) && !state.cluster.clockConverged) ||
                    !Number.isInteger(rate) ||
                    rate < 1 ||
                    rate > 20
                ) return false

                set(
                    { simulationRatePhase: 'pending', lastError: null },
                    false,
                    'control-plane/simulation-rate-started',
                )
                try {
                    const receipt = await updateSimulationRate(
                        rate,
                        state.cluster.clockResourceVersion,
                    )
                    set(
                        {
                            cluster: {
                                ...get().cluster,
                                clockRate: receipt.rate,
                                clockResourceVersion: receipt.resourceVersion,
                                clockConverged: receipt.convergence === 'converged',
                            },
                            simulationRatePhase: 'success',
                        },
                        false,
                        'control-plane/simulation-rate-succeeded',
                    )
                    return true
                } catch (error) {
                    set(
                        {
                            simulationRatePhase: 'error',
                            lastError: errorMessage(error),
                        },
                        false,
                        'control-plane/simulation-rate-failed',
                    )
                    return false
                }
            },

            clearDistributionFeedback: () => {
                set(
                    { distributionPhase: 'idle' },
                    false,
                    'control-plane/distribution-feedback-cleared',
                )
            },

            setExecutionMode: (executionMode) => {
                const state = get()
                if (executionMode === 'test' && !canRunTest(state.cluster)) return false

                set(
                    {
                        executionMode,
                        executionPhase: executionMode === 'test' ? 'running' : 'standby',
                        lastError: null,
                    },
                    false,
                    `control-plane/mode-${executionMode}`,
                )
                return true
            },

            forceApplyMode: () => {
                const state = get()
                if (state.executionMode === 'apply' && state.executionPhase === 'standby') return
                set(
                    { executionMode: 'apply', executionPhase: 'standby' },
                    false,
                    'control-plane/mode-forced-apply',
                )
            },
        }),
        { name: 'control-plane-store' },
    ),
)
