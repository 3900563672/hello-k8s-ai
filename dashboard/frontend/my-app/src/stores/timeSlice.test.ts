import { beforeEach, describe, expect, it } from 'vitest'
import {
    getReplayTimeContext,
    useTimeStore,
} from '@/stores/timeSlice'
import type { Snapshot } from '@/types/time.types'

const makeSnapshot = (
    id: string,
    timestamp: string,
    overrides: Partial<Snapshot> = {},
): Snapshot => ({
    id,
    timestamp,
    weight: 1,
    type: 'config',
    trigger: 'time',
    domain: 'configuration',
    severity: 'normal',
    title: id,
    summary: '',
    source: 'postgresql-snapshot',
    impact: { tenants: 0, nodes: 0, models: 0, changes: 0 },
    tags: [],
    ...overrides,
})

const T0 = '2026-08-12T12:00:00.000Z'
const T1 = '2026-08-12T12:01:00.000Z'
const T2 = '2026-08-12T12:02:00.000Z'

describe('timeSlice', () => {
    beforeEach(() => {
        useTimeStore.setState(useTimeStore.getInitialState())
    })

    it('setSnapshots 排序并选中最新快照（latest 模式）', () => {
        const snapshots = [
            makeSnapshot('b', T1),
            makeSnapshot('c', T2),
            makeSnapshot('a', T0),
        ]
        useTimeStore.getState().setSnapshots(snapshots)
        const state = useTimeStore.getState()
        expect(state.snapshots.map((item) => item.id)).toEqual(['a', 'b', 'c'])
        expect(state.selectedSnapshotId).toBe('c')
        expect(state.mode).toBe('latest')
        expect(state.timestamp).toBe(T2)
        expect(state.revision).toBe(1)
    })

    it('setSnapshots 内容未变化时不更新 revision，避免轮询重渲染', () => {
        const snapshots = [makeSnapshot('a', T0), makeSnapshot('b', T1)]
        useTimeStore.getState().setSnapshots(snapshots)
        const revision = useTimeStore.getState().revision
        useTimeStore.getState().setSnapshots([...snapshots].reverse())
        expect(useTimeStore.getState().revision).toBe(revision)
    })

    it('selectSnapshot 切到历史快照并保持 latest 选中最后一项', () => {
        useTimeStore.getState().setSnapshots([
            makeSnapshot('a', T0),
            makeSnapshot('b', T1),
        ])
        useTimeStore.getState().selectSnapshot('a')
        let state = useTimeStore.getState()
        expect(state.selectedSnapshotId).toBe('a')
        expect(state.mode).toBe('historical')
        expect(state.timestamp).toBe(T0)

        useTimeStore.getState().selectSnapshot('b')
        state = useTimeStore.getState()
        expect(state.selectedSnapshotId).toBe('b')
        expect(state.mode).toBe('latest')
    })

    it('selectSnapshot 未知 id 不改变状态', () => {
        useTimeStore.getState().setSnapshots([makeSnapshot('a', T0)])
        const before = useTimeStore.getState()
        useTimeStore.getState().selectSnapshot('missing')
        const after = useTimeStore.getState()
        expect(after.selectedSnapshotId).toBe(before.selectedSnapshotId)
        expect(after.revision).toBe(before.revision)
    })

    it('jumpToNearest 选择时间距离最近的快照', () => {
        useTimeStore.getState().setSnapshots([
            makeSnapshot('a', T0),
            makeSnapshot('b', T1),
            makeSnapshot('c', T2),
        ])
        useTimeStore.getState().jumpToNearest('2026-08-12T12:01:30.000Z')
        expect(useTimeStore.getState().selectedSnapshotId).toBe('b')
        useTimeStore.getState().jumpToNearest('2026-08-12T12:00:01.000Z')
        expect(useTimeStore.getState().selectedSnapshotId).toBe('a')
    })

    it('jumpToTimestamp 使用“目标时刻之前最后一个快照”，早于首条则不变', () => {
        useTimeStore.getState().setSnapshots([
            makeSnapshot('a', T0),
            makeSnapshot('b', T1),
        ])
        useTimeStore.getState().jumpToTimestamp('2026-08-12T12:00:30.000Z')
        expect(useTimeStore.getState().selectedSnapshotId).toBe('a')
        // 实现行为：目标早于时间线起点时钳制到第一条快照
        // （timelineMath 注释描述为“目标之前最后一个”，实现返回 snapshots[0]，此处按实现固化）。
        useTimeStore.getState().jumpToTimestamp('2026-08-12T11:00:00.000Z')
        expect(useTimeStore.getState().selectedSnapshotId).toBe('a')
        expect(useTimeStore.getState().mode).toBe('historical')
    })

    it('stepSnapshot 从当前选中步进并夹紧边界', () => {
        useTimeStore.getState().setSnapshots([
            makeSnapshot('a', T0),
            makeSnapshot('b', T1),
            makeSnapshot('c', T2),
        ])
        useTimeStore.getState().selectSnapshot('a')
        useTimeStore.getState().stepSnapshot(1)
        expect(useTimeStore.getState().selectedSnapshotId).toBe('b')
        useTimeStore.getState().stepSnapshot(1)
        expect(useTimeStore.getState().selectedSnapshotId).toBe('c')
        useTimeStore.getState().stepSnapshot(1)
        expect(useTimeStore.getState().selectedSnapshotId).toBe('c')
        useTimeStore.getState().stepSnapshot(-1)
        expect(useTimeStore.getState().selectedSnapshotId).toBe('b')
    })

    it('stepSnapshot 无选中时向前取第一条、向后取最后一条', () => {
        useTimeStore.getState().setSnapshots([
            makeSnapshot('a', T0),
            makeSnapshot('b', T1),
            makeSnapshot('c', T2),
        ])
        useTimeStore.setState({ selectedSnapshotId: null })
        useTimeStore.getState().stepSnapshot(1)
        expect(useTimeStore.getState().selectedSnapshotId).toBe('a')
        useTimeStore.setState({ selectedSnapshotId: null })
        useTimeStore.getState().stepSnapshot(-1)
        expect(useTimeStore.getState().selectedSnapshotId).toBe('c')
    })

    it('returnToLatest 回到最后一条并置为 latest', () => {
        useTimeStore.getState().setSnapshots([
            makeSnapshot('a', T0),
            makeSnapshot('b', T1),
        ])
        useTimeStore.getState().selectSnapshot('a')
        useTimeStore.getState().returnToLatest()
        const state = useTimeStore.getState()
        expect(state.selectedSnapshotId).toBe('b')
        expect(state.mode).toBe('latest')
    })

    it('setViewport 夹紧到时间线边界且保持跨度', () => {
        useTimeStore.getState().setSnapshots([
            makeSnapshot('a', T0),
            makeSnapshot('b', T1),
        ])
        const bounds = { start: Date.parse(T0), end: Date.parse(T1) }
        useTimeStore.getState().setViewport({ start: bounds.start - 5000, end: bounds.start + 5000 })
        const viewport = useTimeStore.getState().viewport
        expect(viewport.start).toBe(bounds.start)
        expect(viewport.end).toBe(bounds.start + 10000)
        expect(viewport.end).toBeLessThanOrEqual(bounds.end)
    })

    it('focusDuration 传入 null 或大于全跨度时回到完整边界', () => {
        useTimeStore.getState().setSnapshots([
            makeSnapshot('a', T0),
            makeSnapshot('b', T1),
        ])
        const bounds = { start: Date.parse(T0), end: Date.parse(T1) }
        useTimeStore.getState().focusDuration(null)
        expect(useTimeStore.getState().viewport).toEqual(bounds)
        useTimeStore.getState().focusDuration(7_200_000)
        expect(useTimeStore.getState().viewport).toEqual(bounds)
    })

    it('focusDuration 以选中快照为中心收窄视窗', () => {
        useTimeStore.getState().setSnapshots([
            makeSnapshot('a', T0),
            makeSnapshot('b', T1),
        ])
        useTimeStore.getState().selectSnapshot('b')
        useTimeStore.getState().focusDuration(60_000)
        const viewport = useTimeStore.getState().viewport
        expect(viewport.end - viewport.start).toBe(60_000)
        expect(viewport.end).toBe(Date.parse(T1))
    })

    it('revealSelected 在选中项超出视窗时移动视窗', () => {
        useTimeStore.getState().setSnapshots([
            makeSnapshot('a', T0),
            makeSnapshot('b', T1),
            makeSnapshot('c', T2),
        ])
        useTimeStore.getState().setViewport({
            start: Date.parse(T0) - 5000,
            end: Date.parse(T0) + 5000,
        })
        useTimeStore.getState().selectSnapshot('c')
        const viewport = useTimeStore.getState().viewport
        expect(viewport.end).toBe(Date.parse(T2))
        expect(viewport.end - viewport.start).toBe(10_000)
    })

    it('setLatestServerTime 只在空时间线首次设置权威时间', () => {
        const store = useTimeStore.getState()
        expect(store.timestamp).toBe('1970-01-01T00:00:00.000Z')
        store.setLatestServerTime(T1)
        expect(useTimeStore.getState().timestamp).toBe(T1)

        useTimeStore.getState().setLatestServerTime(T2)
        expect(useTimeStore.getState().timestamp).toBe(T1)

        useTimeStore.getState().setSnapshots([makeSnapshot('a', T0)])
        useTimeStore.getState().setLatestServerTime(T2)
        expect(useTimeStore.getState().timestamp).toBe(T0)
    })

    it('getReplayTimeContext 汇总当前回放上下文', () => {
        useTimeStore.getState().setSnapshots([
            makeSnapshot('a', T0),
            makeSnapshot('b', T1),
        ])
        useTimeStore.getState().selectSnapshot('a')
        const context = getReplayTimeContext()
        expect(context).toEqual({
            effectiveAt: T0,
            snapshotId: 'a',
            mode: 'historical',
            revision: 2,
        })
    })
})
