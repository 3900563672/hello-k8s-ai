import { beforeEach, describe, expect, it } from 'vitest'
import { useTrafficStore } from '@/stores/trafficSlice'
import type { OverlayInstance, TrafficPoint } from '@/types/traffic.types'

const makeOverlay = (overrides: Partial<OverlayInstance> = {}): OverlayInstance => ({
    id: 'overlay-1',
    templateId: 'template-1',
    templateName: '模板',
    tenantId: 'tenant-1',
    tenantName: '租户一',
    startOffsetSeconds: 0,
    effectiveAt: '2026-08-20T00:00:00.000Z',
    snapshotId: null,
    enabled: true,
    color: '#5B8CFF',
    createdAt: '2026-08-20T00:00:00.000Z',
    ...overrides,
})

describe('trafficSlice', () => {
    beforeEach(() => {
        useTrafficStore.setState(useTrafficStore.getInitialState())
    })

    it('初始加载预置模板', () => {
        const state = useTrafficStore.getState()
        expect(state.templates.length).toBeGreaterThan(0)
        expect(state.overlays).toEqual([])
        expect(state.mode).toBe('overview')
        expect(state.viewMode).toBe('total')
    })

    it('addTemplate 生成 id、强制 custom 形状并清洗控制点', () => {
        const points: TrafficPoint[] = [
            { x: 120, y: 40 },
            { x: Number.NaN, y: 10 },
            { x: 0, y: -5 },
            { x: 60, y: 20 },
        ]
        const created = useTrafficStore.getState().addTemplate({
            name: '测试模板',
            description: '',
            shapeType: 'custom',
            controlPoints: points,
        })
        expect(created.id).toMatch(/^traffic-/)
        expect(created.shapeType).toBe('custom')
        expect(created.controlPoints).toEqual([
            { x: 0, y: 0 },
            { x: 60, y: 20 },
            { x: 120, y: 40 },
        ])
        expect(Number.isNaN(Date.parse(created.createdAt))).toBe(false)
        expect(Number.isNaN(Date.parse(created.updatedAt))).toBe(false)
    })

    it('updateTemplate 清洗新控制点并保留其余字段', () => {
        const created = useTrafficStore.getState().addTemplate({
            name: '模板',
            shapeType: 'custom',
            controlPoints: [{ x: 0, y: 10 }],
        })
        useTrafficStore.getState().updateTemplate(created.id, {
            name: '改名',
            controlPoints: [{ x: 30, y: 5 }, { x: 10, y: -1 }],
        })
        const updated = useTrafficStore.getState().getTemplate(created.id)
        expect(updated?.name).toBe('改名')
        expect(updated?.shapeType).toBe('custom')
        expect(updated?.controlPoints).toEqual([
            { x: 10, y: 0 },
            { x: 30, y: 5 },
        ])
    })

    it('removeTemplate 级联删除关联 Overlay', () => {
        const created = useTrafficStore.getState().addTemplate({
            name: '模板',
            shapeType: 'custom',
            controlPoints: [{ x: 0, y: 10 }],
        })
        useTrafficStore.getState().addOverlay(makeOverlay({ templateId: created.id }))
        useTrafficStore.getState().removeTemplate(created.id)
        const state = useTrafficStore.getState()
        expect(state.getTemplate(created.id)).toBeUndefined()
        expect(state.overlays).toEqual([])
    })

    it('addOverlay 按顺序轮换颜色并夹紧负偏移', () => {
        const first = useTrafficStore.getState().addOverlay(makeOverlay({ startOffsetSeconds: -10 }))
        const second = useTrafficStore.getState().addOverlay(makeOverlay({ id: 'o2', color: undefined }))
        expect(first.id).toMatch(/^traffic-/)
        expect(first.startOffsetSeconds).toBe(0)
        expect(first.color).toBe('#5B8CFF')
        expect(second.color).toBe('#43C6AC')
    })

    it('toggleOverlay / removeOverlay / clearOverlays 维护启用态', () => {
        useTrafficStore.getState().addOverlay(makeOverlay())
        const overlay = useTrafficStore.getState().overlays[0]
        useTrafficStore.getState().toggleOverlay(overlay.id)
        expect(useTrafficStore.getState().overlays[0].enabled).toBe(false)
        useTrafficStore.getState().toggleOverlay(overlay.id)
        expect(useTrafficStore.getState().overlays[0].enabled).toBe(true)
        useTrafficStore.getState().removeOverlay(overlay.id)
        expect(useTrafficStore.getState().overlays).toHaveLength(0)
        useTrafficStore.getState().addOverlay(makeOverlay())
        useTrafficStore.getState().clearOverlays()
        expect(useTrafficStore.getState().overlays).toHaveLength(0)
    })

    it('getOverlaysForTenant 只返回该租户已启用的 Overlay', () => {
        // addOverlay 始终生成新 id，按添加顺序引用。
        useTrafficStore.getState().addOverlay(makeOverlay())
        useTrafficStore.getState().addOverlay(makeOverlay({ tenantId: 'tenant-2' }))
        useTrafficStore.getState().addOverlay(makeOverlay())
        const third = useTrafficStore.getState().overlays[2]
        useTrafficStore.getState().toggleOverlay(third.id)
        const forTenant = useTrafficStore.getState().getOverlaysForTenant('tenant-1')
        expect(forTenant).toHaveLength(1)
        expect(forTenant[0].tenantId).toBe('tenant-1')
        expect(forTenant[0].enabled).toBe(true)
    })

    it('compareTenants 去重与切换', () => {
        useTrafficStore.getState().setCompareTenants(['a', 'b', 'a'])
        expect(useTrafficStore.getState().compareTenants).toEqual(['a', 'b'])
        useTrafficStore.getState().toggleCompareTenant('c')
        useTrafficStore.getState().toggleCompareTenant('a')
        expect(useTrafficStore.getState().compareTenants).toEqual(['b', 'c'])
    })

    it('视图与工作区模式切换', () => {
        useTrafficStore.getState().setViewMode('single')
        useTrafficStore.getState().setSelectedTenant('tenant-1')
        useTrafficStore.getState().setMode('draw')
        const state = useTrafficStore.getState()
        expect(state.viewMode).toBe('single')
        expect(state.selectedTenant).toBe('tenant-1')
        expect(state.mode).toBe('draw')
    })
})
