import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TrafficPage } from '@/components/features/traffic/TrafficPage'
import { useTrafficStore } from '@/stores/trafficSlice'
import { useTimeStore } from '@/stores/timeSlice'
import type { OverlayInstance, TrafficTemplate } from '@/types/traffic.types'

const { mutateApply } = vi.hoisted(() => ({ mutateApply: vi.fn() }))

vi.mock('echarts-for-react', () => ({
    default: () => <div data-testid="traffic-chart" />,
}))

vi.mock('@/components/features/traffic/DrawCanvas', () => ({
    DrawCanvas: ({ onCancel, onSave }: {
        onCancel: () => void
        onSave: (name: string, points: Array<{ x: number; y: number }>) => void
    }) => (
        <div>
            绘制画布
            <button type="button" onClick={onCancel}>取消</button>
            <button type="button" onClick={() => onSave('自定义曲线', [{ x: 0, y: 0 }, { x: 10, y: 50 }])}>保存模板</button>
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
    useSetTenantTraffic: () => ({ mutate: mutateApply, isPending: false }),
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
        mutateApply.mockReset()
        mutateApply.mockImplementation(
            (_payload: unknown, options?: { onSuccess?: () => void; onError?: (error: unknown) => void }) => {
                options?.onSuccess?.()
            },
        )
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

    it('draw 模式保存模板后回到总览并提示', async () => {
        const user = userEvent.setup()
        render(<TrafficPage />)
        await user.click(screen.getByRole('button', { name: /绘制新模板/ }))
        await user.click(screen.getByRole('button', { name: /保存模板/ }))
        expect(await screen.findByText('流量布置')).toBeInTheDocument()
        expect(screen.getByText(/模板“自定义曲线”已保存/)).toBeInTheDocument()
        expect(useTrafficStore.getState().templates).toHaveLength(2)
    })

    it('模板库点击卡片打开叠加弹窗，确认后写入目标 QPS', async () => {
        const user = userEvent.setup()
        render(<TrafficPage />)
        fireEvent.click(screen.getAllByText('脉冲峰值')[0])
        await user.click(screen.getByRole('combobox'))
        await user.click(await screen.findByRole('option', { name: /租户A/ }))
        await user.type(screen.getByLabelText(/逻辑开始偏移/), '30')
        await user.click(screen.getByRole('button', { name: /确认叠加/ }))
        await waitFor(() => expect(mutateApply).toHaveBeenCalled())
        expect(mutateApply).toHaveBeenCalledWith(
            { tenantId: 'tenant-a', qps: 100 },
            expect.anything(),
        )
        expect(screen.getByText(/已将“脉冲峰值”叠加到 租户A，目标 QPS 已写入（100）/)).toBeInTheDocument()
        expect(useTrafficStore.getState().overlays).toHaveLength(2)
    })

    it('历史模式只读：确认叠加被拒绝且不调用写入', async () => {
        useTimeStore.setState({ mode: 'historical', snapshots: [], selectedSnapshotId: null })
        const user = userEvent.setup()
        render(<TrafficPage />)
        fireEvent.click(screen.getAllByText('脉冲峰值')[0])
        await user.click(screen.getByRole('combobox'))
        await user.click(await screen.findByRole('option', { name: /租户A/ }))
        await user.type(screen.getByLabelText(/逻辑开始偏移/), '30')
        await user.click(screen.getByRole('button', { name: /确认叠加/ }))
        expect(screen.getByText('历史模式只读，不能应用流量')).toBeInTheDocument()
        expect(mutateApply).not.toHaveBeenCalled()
    })

    it('写入失败展示错误提示', async () => {
        mutateApply.mockImplementation(
            (_payload: unknown, options?: { onError?: (error: unknown) => void }) => {
                options?.onError?.(new Error('后端拒绝'))
            },
        )
        const user = userEvent.setup()
        render(<TrafficPage />)
        fireEvent.click(screen.getAllByText('脉冲峰值')[0])
        await user.click(screen.getByRole('combobox'))
        await user.click(await screen.findByRole('option', { name: /租户A/ }))
        await user.type(screen.getByLabelText(/逻辑开始偏移/), '30')
        await user.click(screen.getByRole('button', { name: /确认叠加/ }))
        expect(await screen.findByText('应用失败：后端拒绝')).toBeInTheDocument()
    })

    it('租户对比视图：徽标选中/取消选中', async () => {
        const user = userEvent.setup()
        render(<TrafficPage />)
        await user.click(screen.getByRole('tab', { name: /租户对比/ }))
        expect(screen.getByText('租户流量对比')).toBeInTheDocument()
        await user.click(screen.getAllByText('租户A')[0])
        expect(useTrafficStore.getState().compareTenants).toContain('tenant-a')
        await user.click(screen.getAllByText('租户A')[0])
        expect(useTrafficStore.getState().compareTenants).not.toContain('tenant-a')
    })

    it('单租户视图选择租户后标题更新', async () => {
        const user = userEvent.setup()
        render(<TrafficPage />)
        await user.click(screen.getByRole('tab', { name: /单租户/ }))
        await user.click(screen.getByRole('combobox'))
        await user.click(await screen.findByRole('option', { name: /租户A/ }))
        expect(useTrafficStore.getState().selectedTenant).toBe('tenant-a')
        expect(screen.getAllByText('租户A').length).toBeGreaterThanOrEqual(2)
    })
})