import { describe, expect, it } from 'vitest'
import {
    buildScenarioTimePoints,
    formatLogicalTime,
    formatQps,
    getOverlayEndSeconds,
    getScenarioHorizon,
    getTemplateDuration,
    getTemplatePeakQps,
    getTemplateValueAtTime,
    getTenantSeriesValues,
    getTotalSeriesValues,
    sanitizeControlPoints,
} from '@/components/features/traffic/trafficMath'
import type { OverlayInstance, TrafficTemplate } from '@/types/traffic.types'

const makeTemplate = (id: string, controlPoints: TrafficTemplate['controlPoints']): TrafficTemplate => ({
    id,
    name: id,
    shapeType: 'custom',
    controlPoints,
    createdAt: '2026-08-20T00:00:00.000Z',
    updatedAt: '2026-08-20T00:00:00.000Z',
})

const makeOverlay = (overrides: Partial<OverlayInstance>): OverlayInstance => ({
    id: 'overlay-1',
    templateId: 'template-a',
    templateName: 'a',
    tenantId: 'tenant-1',
    tenantName: 't1',
    startOffsetSeconds: 0,
    effectiveAt: '2026-08-20T00:00:00.000Z',
    snapshotId: null,
    enabled: true,
    color: '#5B8CFF',
    createdAt: '2026-08-20T00:00:00.000Z',
    ...overrides,
})

describe('sanitizeControlPoints', () => {
    it('过滤非法、钳制负数、排序并去重同 x 点', () => {
        const points = sanitizeControlPoints([
            { x: 10, y: 5 },
            { x: Number.NaN, y: 1 },
            { x: 0, y: -3 },
            { x: 10, y: 9 },
            { x: 5, y: 2 },
        ])
        expect(points).toEqual([
            { x: 0, y: 0 },
            { x: 5, y: 2 },
            { x: 10, y: 9 },
        ])
    })
})

describe('模板曲线工具', () => {
    const template = makeTemplate('template-a', [
        { x: 0, y: 0 },
        { x: 60, y: 100 },
        { x: 120, y: 0 },
    ])

    it('getTemplateDuration 取最后控制点 x', () => {
        expect(getTemplateDuration(template)).toBe(120)
        expect(getTemplateDuration(makeTemplate('empty', []))).toBe(0)
    })

    it('getTemplatePeakQps 取最大有限 y', () => {
        expect(getTemplatePeakQps(template)).toBe(100)
        expect(getTemplatePeakQps(makeTemplate('nan', [{ x: 0, y: Number.NaN }]))).toBe(0)
    })

    it('getTemplateValueAtTime 线性插值，区间外返回 0', () => {
        expect(getTemplateValueAtTime(template, 0)).toBe(0)
        expect(getTemplateValueAtTime(template, 30)).toBe(50)
        expect(getTemplateValueAtTime(template, 90)).toBe(50)
        expect(getTemplateValueAtTime(template, 120)).toBe(0)
        expect(getTemplateValueAtTime(template, -1)).toBe(0)
        expect(getTemplateValueAtTime(template, 121)).toBe(0)
    })
})

describe('场景坐标与聚合', () => {
    const template = makeTemplate('template-a', [{ x: 0, y: 10 }, { x: 60, y: 20 }])
    const overlay = makeOverlay({ startOffsetSeconds: 30 })

    it('getOverlayEndSeconds 计算偏移 + 模板时长', () => {
        expect(getOverlayEndSeconds(overlay, [template])).toBe(90)
        expect(getOverlayEndSeconds(makeOverlay({ startOffsetSeconds: -5 }), [template])).toBe(60)
        expect(getOverlayEndSeconds(overlay, [])).toBe(30)
    })

    it('getScenarioHorizon 至少为最小视界，超长时加 8% 内边距', () => {
        expect(getScenarioHorizon([template], [overlay], 300)).toBe(300)
        const longOverlay = makeOverlay({ startOffsetSeconds: 7200 })
        expect(getScenarioHorizon([template], [longOverlay], 300)).toBeGreaterThan(7200)
        const disabled = makeOverlay({ startOffsetSeconds: 9999, enabled: false })
        expect(getScenarioHorizon([template], [disabled], 300)).toBe(300)
    })

    it('buildScenarioTimePoints 包含 0/horizon 与叠加曲线节点', () => {
        const points = buildScenarioTimePoints([template], [overlay], 300)
        expect(points[0]).toBe(0)
        expect(points.at(-1)).toBe(300)
        expect(points).toContain(30)
        expect(points).toContain(90)
        expect([...points].sort((a, b) => a - b)).toEqual(points)
    })

    it('getTenantSeriesValues 只叠加该租户已启用 Overlay 并夹紧非负', () => {
        const timePoints = [0, 30, 60, 90]
        const values = getTenantSeriesValues(
            'tenant-1',
            timePoints,
            [template],
            [
                overlay,
                makeOverlay({ id: 'other-tenant', tenantId: 'tenant-2' }),
                makeOverlay({ id: 'disabled', enabled: false }),
            ],
        )
        // t=30 → 模板 0s=10；t=60 → 模板 30s=15；t=90 → 模板 60s=20
        expect(values).toEqual([0, 10, 15, 20])
    })

    it('getTotalSeriesValues 跨租户求和', () => {
        const values = getTotalSeriesValues(
            ['tenant-1', 'tenant-2'],
            [30, 60],
            [template],
            [
                overlay,
                makeOverlay({ id: 't2', tenantId: 'tenant-2', startOffsetSeconds: 0 }),
            ],
        )
        // t=30：tenant-1 模板 0s=10 + tenant-2 模板 30s=15 = 25；t=60：15 + 20 = 35
        expect(values).toEqual([25, 35])
    })
})

describe('格式化', () => {
    it('formatLogicalTime 输出 h/m/s', () => {
        expect(formatLogicalTime(0)).toBe('0s')
        expect(formatLogicalTime(45)).toBe('45s')
        expect(formatLogicalTime(90)).toBe('1m 30s')
        expect(formatLogicalTime(3661)).toBe('1h 1m 1s')
        expect(formatLogicalTime(7200)).toBe('2h')
        expect(formatLogicalTime(2.5)).toBe('2.5s')
        expect(formatLogicalTime(Number.POSITIVE_INFINITY)).toBe('0s')
    })

    it('formatQps 千/百万/十亿缩写与四舍五入', () => {
        expect(formatQps(999)).toBe('999')
        expect(formatQps(1500)).toBe('1.5k')
        expect(formatQps(2_000_000)).toBe('2M')
        expect(formatQps(1_500_000_000)).toBe('1.5B')
        expect(formatQps(-5)).toBe('0')
        expect(formatQps(Number.NaN)).toBe('0')
    })
})
