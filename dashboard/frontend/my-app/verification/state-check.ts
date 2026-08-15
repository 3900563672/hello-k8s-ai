import assert from 'node:assert/strict'
import {
    canRunTest,
    onlineWorkerCount,
    useControlPlaneStore,
} from '@/stores/controlPlaneSlice'
import { useTimeStore } from '@/stores/timeSlice'
import type { Snapshot } from '@/types/time.types'

const snapshots: Snapshot[] = [
    {
        id: 'resource-event-1',
        timestamp: '2026-08-12T12:00:00.000Z',
        weight: 2,
        type: 'event',
        trigger: 'event',
        domain: 'runtime',
        severity: 'normal',
        title: 'SimulatorInstance modified',
        summary: 'Resource version 101 observed by informer.',
        source: 'kubernetes-informer',
        impact: { tenants: 1, nodes: 0, models: 1, changes: 1 },
        tags: ['SimulatorInstance', 'MODIFIED'],
    },
    {
        id: 'snapshot-2',
        timestamp: '2026-08-12T12:01:00.000Z',
        weight: 1,
        type: 'config',
        trigger: 'time',
        domain: 'configuration',
        severity: 'normal',
        title: 'Kubernetes snapshot',
        summary: 'Periodic informer read-model snapshot.',
        source: 'postgresql-snapshot',
        impact: { tenants: 2, nodes: 3, models: 2, changes: 0 },
        tags: ['snapshot'],
    },
]

useTimeStore.getState().setSnapshots(snapshots)
let state = useTimeStore.getState()
assert.equal(state.snapshots.length, 2)
assert.equal(state.selectedSnapshotId, 'snapshot-2')
assert.equal(state.mode, 'latest')

state.jumpToTimestamp(snapshots[0].timestamp)
state = useTimeStore.getState()
assert.equal(state.selectedSnapshotId, 'resource-event-1')
assert.equal(state.mode, 'historical')

state.returnToLatest()
state = useTimeStore.getState()
assert.equal(state.selectedSnapshotId, 'snapshot-2')
assert.equal(state.mode, 'latest')

const control = useControlPlaneStore.getState()
assert.equal(onlineWorkerCount(control.cluster), 0)
assert.equal(canRunTest(control.cluster), false)
assert.equal(control.setExecutionMode('test'), false)
assert.equal(useControlPlaneStore.getState().executionMode, 'apply')
assert.equal(await control.setSimulationRate(2), false)
assert.equal(useControlPlaneStore.getState().simulationRatePhase, 'idle')

const originalFetch = globalThis.fetch
let rateRequest: { url: string; init?: RequestInit } | null = null
useControlPlaneStore.setState((current) => ({
    cluster: {
        ...current.cluster,
        connectionStatus: 'connected',
        simulationRateSupported: true,
        clockResourceVersion: '17',
        clockConverged: true,
    },
}))
globalThis.fetch = async (input, init) => {
    rateRequest = { url: String(input), init }
    return new Response(JSON.stringify({
        data: {
            results: [{ resourceVersion: '18', convergence: 'pending' }],
        },
        meta: {
            requestId: 'request-rate',
            servedAt: '2026-08-14T12:02:00.000Z',
            partial: false,
            warnings: [],
            sourceVersions: { kubernetes: '18' },
        },
    }), {
        status: 202,
        headers: { 'Content-Type': 'application/json' },
    })
}
try {
    assert.equal(await useControlPlaneStore.getState().setSimulationRate(5), true)
} finally {
    globalThis.fetch = originalFetch
}
const rateState = useControlPlaneStore.getState()
assert.equal(rateState.cluster.clockRate, 5)
assert.equal(rateState.cluster.clockResourceVersion, '18')
assert.equal(rateState.cluster.clockConverged, false)
assert.equal(rateState.simulationRatePhase, 'success')
assert.ok(rateRequest)
assert.equal(rateRequest.url, '/api/v1/clock/rate')
assert.equal(rateRequest.init?.method, 'PATCH')
assert.deepEqual(JSON.parse(String(rateRequest.init?.body)), {
    rate: 5,
    resourceVersion: '17',
    dryRun: false,
})
assert.ok(new Headers(rateRequest.init?.headers).get('Idempotency-Key')?.startsWith('simulation-rate-'))

console.log(JSON.stringify({
    timelineItems: state.snapshots.length,
    selectedSnapshot: state.selectedSnapshotId,
    initialOnlineWorkers: onlineWorkerCount(control.cluster),
    simulationRunSupported: control.cluster.simulationRunSupported,
    simulationRateSupported: rateState.cluster.simulationRateSupported,
    simulationRate: rateState.cluster.clockRate,
}))
