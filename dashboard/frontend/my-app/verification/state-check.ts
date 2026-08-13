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

console.log(JSON.stringify({
    timelineItems: state.snapshots.length,
    selectedSnapshot: state.selectedSnapshotId,
    initialOnlineWorkers: onlineWorkerCount(control.cluster),
    simulationRunSupported: control.cluster.simulationRunSupported,
}))
