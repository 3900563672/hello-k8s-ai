import { describe, expect, it, beforeEach, afterEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useControlPlaneStore } from '@/stores/controlPlaneSlice'
import { useTimeStore } from '@/stores/timeSlice'
import { useWorkspaceContext, useWorkspaceCoordinator } from '@/hooks/useWorkspaceContext'
import { resetReplayStore } from '@/test/queryUtils'
import type { ClusterSnapshot } from '@/types/control-plane.types'
import type { Snapshot } from '@/types/time.types'

const makeCluster = (overrides: Partial<ClusterSnapshot> = {}): ClusterSnapshot => ({
    ...useControlPlaneStore.getInitialState().cluster,
    connectionStatus: 'connected',
    simulationRunSupported: true,
    simulationRateSupported: true,
    clockResourceVersion: '',
    clockConverged: false,
    workers: [
        {
            id: 'worker-0',
            name: 'worker-0',
            role: 'worker',
            status: 'running',
            ready: true,
            zone: 'zone-a',
            gpuCapacity: 8,
        },
    ],
    ...overrides,
})

const makeSnapshot = (overrides: Partial<Snapshot> = {}): Snapshot => ({
    id: 'snap-1',
    timestamp: '2026-08-20T00:00:00.000Z',
    weight: 0,
    type: 'config',
    trigger: 'time',
    domain: 'scheduler',
    severity: 'normal',
    title: 's1',
    summary: '',
    source: 'postgresql-snapshot',
    impact: { tenants: 0, nodes: 0, models: 0, changes: 0 },
    tags: [],
    ...overrides,
})

const enterHistorical = (): void => {
    useTimeStore.setState({
        timestamp: '2026-08-20T00:00:00.000Z',
        selectedSnapshotId: 'snap-1',
        mode: 'historical',
        revision: 1,
        snapshots: [makeSnapshot()],
    })
}

describe('useWorkspaceContext', () => {
    beforeEach(() => {
        resetReplayStore()
        useControlPlaneStore.setState({ cluster: makeCluster(), executionMode: 'test', executionPhase: 'running' })
    })
    afterEach(() => {
        useControlPlaneStore.setState(useControlPlaneStore.getInitialState())
    })

    it('latest + 在线 Worker 时可测试且可写', () => {
        const { result } = renderHook(() => useWorkspaceContext())
        expect(result.current.isHistorical).toBe(false)
        expect(result.current.isWritable).toBe(true)
        expect(result.current.canTest).toBe(true)
        expect(result.current.onlineWorkers).toBe(1)
        expect(result.current.totalWorkers).toBe(1)
    })

    it('historical 模式不可写不可测试', () => {
        enterHistorical()
        const { result } = renderHook(() => useWorkspaceContext())
        expect(result.current.isHistorical).toBe(true)
        expect(result.current.isWritable).toBe(false)
        expect(result.current.canTest).toBe(false)
        expect(result.current.selectedSnapshot?.id).toBe('snap-1')
    })

    it('集群不可用时 canTest 为 false', () => {
        useControlPlaneStore.setState({ cluster: makeCluster({ connectionStatus: 'disconnected' }) })
        const { result } = renderHook(() => useWorkspaceContext())
        expect(result.current.canTest).toBe(false)
    })
})

describe('useWorkspaceCoordinator', () => {
    beforeEach(() => {
        resetReplayStore()
        useControlPlaneStore.setState({ cluster: makeCluster(), executionMode: 'test', executionPhase: 'running' })
    })
    afterEach(() => {
        useControlPlaneStore.setState(useControlPlaneStore.getInitialState())
    })

    it('historical 时把 test 模式强制复位为 apply', () => {
        enterHistorical()
        renderHook(() => useWorkspaceCoordinator())
        expect(useControlPlaneStore.getState().executionMode).toBe('apply')
        expect(useControlPlaneStore.getState().executionPhase).toBe('standby')
    })

    it('集群不可用时 test 模式强制复位为 apply', () => {
        useControlPlaneStore.setState({ cluster: makeCluster({ connectionStatus: 'disconnected' }) })
        renderHook(() => useWorkspaceCoordinator())
        expect(useControlPlaneStore.getState().executionMode).toBe('apply')
    })

    it('latest + 集群可用时保持 test 模式', () => {
        renderHook(() => useWorkspaceCoordinator())
        expect(useControlPlaneStore.getState().executionMode).toBe('test')
    })
})