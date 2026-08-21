import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { TrafficCanvas } from '@/components/features/traffic/TrafficCanvas'
import { useTrafficStore } from '@/stores/trafficSlice'
import type { OverlayInstance, TrafficTemplate } from '@/types/traffic.types'

let latestOption: unknown = null
vi.mock('echarts-for-react', () => ({
    default: ({ option }: { option: unknown }) => {
        latestOption = option
        return <div data-testid="traffic-chart" />
    },
}))

vi.mock('@/api/queries/trafficQueries', () => ({
    useTenants: () => ({
        data: [
            { id: 'tenant-a', name: '租户A', priority: 'P1', requestedQPS: 100 },
            { id: 'tenant-b', name: '租户B', priority: 'P2', requestedQPS: 50 },
        ],
    }),
}))

const template: TrafficTemplate = {
    id: 'tpl-1',
    name: '脉冲峰值',
    shapeType: 'spike',
    controlPoints: [
        { x: 0, y: 0 },
        { x: 60, y: 50 },
        { x: 120, y: 0 },
    ],
    createdAt: '2026-01-01T00:00:00.000Z',
    updatedAt: '2026-01-01T00:00:00.000Z',
}

const overlay: OverlayInstance = {
    id: 'ov-1',
    templateId: 'tpl-1',
    templateName: '脉冲峰值',
    tenantId: 'tenant-a',
    tenantName: '租户A',
    startOffsetSeconds: 30,
    effectiveAt: '2026-01-01T00:00:00.000Z',
    snapshotId: null,
    enabled: true,
    color: '#6DA6FF',
    createdAt: '2026-01-01T00:00:00.000Z',
}

describe('TrafficCanvas', () => {
    beforeEach(() => {
        latestOption = null
        useTrafficStore.setState({
            viewMode: 'total',
            selectedTenant: null,
            compareTenants: [],
            templates: [template],
            overlays: [overlay],
        })
    })

    it('total 视图渲染总 QPS 曲线与空态标题', () => {
        render(<TrafficCanvas />)
        expect(screen.getByTestId('traffic-chart')).toBeInTheDocument()
        expect(latestOption).not.toBeNull()
        expect(JSON.stringify(latestOption)).toContain('总 QPS')
    })

    it('total 视图无叠加时展示引导空态', () => {
        useTrafficStore.setState({ overlays: [] })
        render(<TrafficCanvas />)
        expect(screen.getByText('还没有布置流量')).toBeInTheDocument()
        expect(screen.getByText(/从左侧选择模板/)).toBeInTheDocument()
    })

    it('single 视图未选租户时提示选择', () => {
        useTrafficStore.setState({ viewMode: 'single', selectedTenant: null })
        render(<TrafficCanvas />)
        expect(screen.getByText('请选择一个租户')).toBeInTheDocument()
    })

    it('single 视图选中租户渲染其曲线', () => {
        useTrafficStore.setState({ viewMode: 'single', selectedTenant: 'tenant-a' })
        render(<TrafficCanvas />)
        expect(JSON.stringify(latestOption)).toContain('租户A')
    })

    it('compare 视图未选租户时提示选择对比', () => {
        useTrafficStore.setState({ viewMode: 'compare', compareTenants: [] })
        render(<TrafficCanvas />)
        expect(screen.getByText('请选择要对比的租户')).toBeInTheDocument()
    })
})