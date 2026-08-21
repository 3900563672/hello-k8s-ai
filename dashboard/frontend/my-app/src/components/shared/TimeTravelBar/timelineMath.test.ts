import { describe, expect, it } from 'vitest'
import {
    aggregateSnapshots,
    chooseGranularity,
    clampViewport,
    findNearestSnapshot,
    findSnapshotAtOrBefore,
    getTimelineBounds,
    snapshotTime,
    sortSnapshots,
    viewportEquals,
} from '@/components/shared/TimeTravelBar/timelineMath'
import type { Snapshot } from '@/types/time.types'

const makeSnapshot = (
    id: string,
    timestamp: string,
    trigger: 'event' | 'time' = 'time',
    weight = 1,
): Snapshot => ({
    id,
    timestamp,
    weight,
    type: trigger === 'event' ? 'event' : 'config',
    trigger,
    domain: 'configuration',
    severity: 'normal',
    title: id,
    summary: '',
    source: 'postgresql-snapshot',
    impact: { tenants: 0, nodes: 0, models: 0, changes: 0 },
    tags: [],
})

const T0 = '2026-08-12T12:00:00.000Z'
const T1 = '2026-08-12T12:01:00.000Z'
const T2 = '2026-08-12T12:02:00.000Z'

describe('timelineMath', () => {
    it('snapshotTime 解析快照对象的时间戳，非法返回 0', () => {
        expect(snapshotTime({ timestamp: T0 })).toBe(Date.parse(T0))
        // 实现（validTime）兼容 number 时间戳；类型签名为 string，故断言类型
expect(snapshotTime({ timestamp: Date.parse(T0) } as unknown as Parameters<typeof snapshotTime>[0])).toBe(Date.parse(T0))
        expect(snapshotTime({ timestamp: 'bad' })).toBe(0)
    })

    it('sortSnapshots 按时间升序且不修改输入', () => {
        const input = [makeSnapshot('b', T1), makeSnapshot('a', T0)]
        const sorted = sortSnapshots(input)
        expect(sorted.map((item) => item.id)).toEqual(['a', 'b'])
        expect(input.map((item) => item.id)).toEqual(['b', 'a'])
    })

    it('getTimelineBounds 空列表回退最近一小时窗口', () => {
        const bounds = getTimelineBounds([])
        expect(bounds.end - bounds.start).toBe(3_600_000)
        expect(Math.abs(bounds.end - Date.now())).toBeLessThan(5_000)
    })

    it('getTimelineBounds 单点时间回退 ±30 秒，多点取首尾', () => {
        const single = getTimelineBounds([makeSnapshot('a', T0)])
        expect(single).toEqual({ start: Date.parse(T0) - 30_000, end: Date.parse(T0) + 30_000 })
        const multi = getTimelineBounds([makeSnapshot('a', T0), makeSnapshot('b', T2)])
        expect(multi).toEqual({ start: Date.parse(T0), end: Date.parse(T2) })
    })

    it('clampViewport 保持跨度并夹紧到边界内', () => {
        const bounds = { start: 0, end: 100_000 }
        expect(clampViewport({ start: -50_000, end: -40_000 }, bounds)).toEqual({ start: 0, end: 10_000 })
        expect(clampViewport({ start: 150_000, end: 160_000 }, bounds)).toEqual({ start: 90_000, end: 100_000 })
        expect(clampViewport({ start: 40_000, end: 50_000 }, bounds)).toEqual({ start: 40_000, end: 50_000 })
    })

    it('clampViewport 请求跨度超过全跨度时收窄到全跨度', () => {
        const bounds = { start: 0, end: 1000 }
        expect(clampViewport({ start: 100, end: 5000 }, bounds)).toEqual({ start: 0, end: 1000 })
    })

    it('clampViewport NaN 输入回退到合法视窗，不产生 NaN 边界（#141）', () => {
        const bounds = { start: 0, end: 100_000 }
        expect(clampViewport({ start: Number.NaN, end: Number.NaN }, bounds)).toEqual({ start: 0, end: 100_000 })
        expect(clampViewport({ start: Number.NaN, end: 50_000 }, bounds)).toEqual({ start: 0, end: 50_000 })
        expect(clampViewport({ start: 10_000, end: Number.NaN }, bounds)).toEqual({ start: 10_000, end: 100_000 })
        expect(clampViewport({ start: Number.NEGATIVE_INFINITY, end: 50_000 }, bounds)).toEqual({ start: 0, end: 50_000 })
    })

    it('viewportEquals 在容差内比较', () => {
        expect(viewportEquals({ start: 0, end: 100 }, { start: 0, end: 100 })).toBe(true)
        expect(viewportEquals({ start: 0, end: 100 }, { start: 1, end: 101 }, 2)).toBe(true)
        expect(viewportEquals({ start: 0, end: 100 }, { start: 5, end: 100 })).toBe(false)
    })

    it('chooseGranularity 视窗越大桶越粗', () => {
        expect(chooseGranularity(60_000, 700).bucketMs).toBeLessThanOrEqual(60_000)
        expect(chooseGranularity(3_600_000, 700).bucketMs).toBe(60_000)
        expect(chooseGranularity(7 * 24 * 3_600_000, 700).bucketMs).toBe(2 * 3_600_000)
        expect(chooseGranularity(30 * 24 * 3_600_000, 700).bucketMs).toBe(12 * 3_600_000)
    })

    it('aggregateSnapshots 分桶统计事件/时间快照与代表 id', () => {
        const bounds = { start: Date.parse(T0), end: Date.parse(T2) }
        const buckets = aggregateSnapshots([
            makeSnapshot('time-a', T0, 'time', 1),
            makeSnapshot('event-b', T0, 'event', 5),
            makeSnapshot('time-c', T1, 'time', 2),
            makeSnapshot('outside', '2026-08-12T13:00:00.000Z', 'time', 9),
        ], bounds, 60_000)
        expect(buckets).toHaveLength(2)
        expect(buckets[0]).toMatchObject({
            start: Date.parse(T0),
            count: 2,
            timeCount: 1,
            eventCount: 1,
            peakWeight: 5,
            representativeId: 'event-b',
        })
        expect(buckets[1]).toMatchObject({ count: 1, timeCount: 1, representativeId: 'time-c' })
    })

    it('findNearestSnapshot 选择距离最近且空列表返回 null', () => {
        const snapshots = [makeSnapshot('a', T0), makeSnapshot('b', T1), makeSnapshot('c', T2)]
        expect(findNearestSnapshot(snapshots, '2026-08-12T12:01:30.000Z')?.id).toBe('b')
        expect(findNearestSnapshot([], T0)).toBeNull()
    })

    it('findSnapshotAtOrBefore 返回目标之前最后一个，早于首条返回 null（与后端 unavailable 语义一致）', () => {
        const snapshots = [makeSnapshot('a', T0), makeSnapshot('b', T1)]
        expect(findSnapshotAtOrBefore(snapshots, T0)?.id).toBe('a')
        expect(findSnapshotAtOrBefore(snapshots, '2026-08-12T12:00:30.000Z')?.id).toBe('a')
        expect(findSnapshotAtOrBefore(snapshots, T2)?.id).toBe('b')
        expect(findSnapshotAtOrBefore(snapshots, '2026-08-12T11:00:00.000Z')).toBeNull()
        expect(findSnapshotAtOrBefore([], T0)).toBeNull()
    })
})
