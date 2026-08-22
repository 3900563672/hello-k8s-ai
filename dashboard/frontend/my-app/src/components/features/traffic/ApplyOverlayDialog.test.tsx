import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ApplyOverlayDialog } from '@/components/features/traffic/ApplyOverlayDialog'
import { useTrafficStore } from '@/stores/trafficSlice'
import { useTimeStore } from '@/stores/timeSlice'
import type { TenantInfo, TrafficTemplate } from '@/types/traffic.types'

vi.mock('@/components/features/traffic/PreviewCanvas', () => ({
    PreviewCanvas: () => <div data-testid="preview-canvas" />,
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

const tenants: TenantInfo[] = [
    { id: 'tenant-a', name: '租户A', priority: 'P1', requestedQPS: 100 },
    { id: 'tenant-b', name: '租户B', priority: 'P2', requestedQPS: 50 },
]

describe('ApplyOverlayDialog', () => {
    const onApply = vi.fn()
    const onOpenChange = vi.fn()

    beforeEach(() => {
        onApply.mockClear()
        onOpenChange.mockClear()
        useTrafficStore.setState({ templates: [template], overlays: [] })
        useTimeStore.setState({
            mode: 'latest',
            timestamp: '2026-01-01T00:00:00.000Z',
            selectedSnapshotId: null,
            revision: 0,
            snapshots: [],
        })
    })

    it('关闭时不渲染对话框内容', () => {
        render(
            <ApplyOverlayDialog
                open={false}
                onOpenChange={onOpenChange}
                template={template}
                tenants={tenants}
                onApply={onApply}
            />,
        )
        expect(screen.queryByText('配置流量叠加')).not.toBeInTheDocument()
    })

    it('打开时展示模板信息、租户选择与预览区', () => {
        render(
            <ApplyOverlayDialog
                open
                onOpenChange={onOpenChange}
                template={template}
                tenants={tenants}
                onApply={onApply}
            />,
        )
        expect(screen.getByText('配置流量叠加')).toBeInTheDocument()
        expect(screen.getByText('脉冲峰值 · 纯 QPS 加法')).toBeInTheDocument()
        expect(screen.getByTestId('preview-canvas')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /确认叠加/ })).toBeDisabled()
    })

    it('填偏移但未选租户时提交保持禁用', async () => {
        const user = userEvent.setup()
        render(
            <ApplyOverlayDialog
                open
                onOpenChange={onOpenChange}
                template={template}
                tenants={tenants}
                onApply={onApply}
            />,
        )
        await user.type(screen.getByLabelText(/逻辑开始偏移/), '30')
        expect(screen.getByRole('button', { name: /确认叠加/ })).toBeDisabled()
        expect(onApply).not.toHaveBeenCalled()
    })

    it('选择租户并填写偏移后提交并关闭', async () => {
        const user = userEvent.setup()
        render(
            <ApplyOverlayDialog
                open
                onOpenChange={onOpenChange}
                template={template}
                tenants={tenants}
                onApply={onApply}
            />,
        )
        await user.click(screen.getByRole('combobox'))
        await user.click(await screen.findByText('租户A'))
        await user.type(screen.getByLabelText(/逻辑开始偏移/), '30')
        await user.click(screen.getByRole('button', { name: /确认叠加/ }))
        expect(onApply).toHaveBeenCalledWith({
            templateId: 'tpl-1',
            tenantId: 'tenant-a',
            startOffsetSeconds: 30,
        })
        expect(onOpenChange).toHaveBeenCalledWith(false)
    })

    it('历史模式禁用提交并提示只读', () => {
        useTimeStore.setState({
            mode: 'historical',
            timestamp: '2026-01-01T00:00:00.000Z',
            selectedSnapshotId: 'snap-1',
            revision: 0,
            snapshots: [],
        })
        render(
            <ApplyOverlayDialog
                open
                onOpenChange={onOpenChange}
                template={template}
                tenants={tenants}
                onApply={onApply}
            />,
        )
        expect(screen.getByText('历史模式只读，不能应用流量')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /确认叠加/ })).toBeDisabled()
    })
})
