import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TrafficPage } from '@/components/features/traffic/TrafficPage'
import { useTrafficStore } from '@/stores/trafficSlice'
import { useTimeStore } from '@/stores/timeSlice'
import type { OverlayInstance, TrafficTemplate } from '@/types/traffic.types'

vi.mock('echarts-for-react', () => ({
    default: () => <div data-testid="traffic-chart" />,
}))

vi.mock('@/components/features/traffic/DrawCanvas', () => ({
    DrawCanvas: ({ onCancel }: { onCancel: () => void }) => (
        <div>
            绘制画布
            <button type="button" onClick={onCancel}>取消</button>
        </div>
    ),
}))

vi.mock('@/api/queries/trafficQueries', () => ({
    useTenants: () => ({
        data: [
            { id: 'tenant-a', name: '租户A', priority: 'P1', requestedQPS: 100 },
            { id: 'tenant-b', name: '租户B', priority: 'P2', requestedQPS: 50 },
        ],
    }),
    useSetTenantTraffic: () => ({ mutate: vi.fn(), isPending: false }),
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

describe('TrafficPage', () => {
    beforeEach(() => {
        useTrafficStore.setState({
            viewMode: 'total',
            selectedTenant: null,
            compareTenants: [],
            templates: [template],
            overlays: [overlay],
            mode: 'overview',
        })
        useTimeStore.setState({ mode: 'latest', timestamp: new Date(0).toISOString(), selectedSnapshotId: null, revision: 0 })
    })

    it('渲染流量布置页头、视图切换与画布', () => {
        render(<TrafficPage />)
        expect(screen.getByText('流量布置')).toBeInTheDocument()
        expect(screen.getByRole('tab', { name: /总 QPS/ })).toBeInTheDocument()
        expect(screen.getByRole('tab', { name: /单租户/ })).toBeInTheDocument()
        expect(screen.getByRole('tab', { name: /租户对比/ })).toBeInTheDocument()
        expect(screen.getByTestId('traffic-chart')).toBeInTheDocument()
        expect(screen.getAllByText('脉冲峰值').length).toBeGreaterThan(0)
    })

    it('切到单租户视图展示租户下拉', async () => {
        const user = userEvent.setup()
        render(<TrafficPage />)
        await user.click(screen.getByRole('tab', { name: /单租户/ }))
        expect(screen.getByText('选择租户（不预选）')).toBeInTheDocument()
    })

    it('draw 模式展示绘制画布并取消返回总览', async () => {
        const user = userEvent.setup()
        render(<TrafficPage />)
        await user.click(screen.getByRole('button', { name: /绘制新模板/ }))
        expect(screen.getByText('绘制画布')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: '取消' }))
        expect(screen.getByText('流量布置')).toBeInTheDocument()
    })

    it('历史模式渲染只读横幅', () => {
        useTimeStore.setState({ mode: 'historical', snapshots: [], selectedSnapshotId: null })
        render(<TrafficPage />)
        expect(screen.getByText('历史基线')).toBeInTheDocument()
    })
})