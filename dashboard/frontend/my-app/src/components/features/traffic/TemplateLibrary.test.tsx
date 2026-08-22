import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DndContext } from '@dnd-kit/core'
import { MemoryRouter } from 'react-router-dom'
import { useTrafficStore } from '@/stores/trafficSlice'
import { TemplateLibrary } from '@/components/features/traffic/TemplateLibrary'
import type { TrafficTemplate } from '@/types/traffic.types'

vi.mock('echarts-for-react', () => ({
    default: () => <div data-testid="echarts" />,
}))

const template: TrafficTemplate = {
    id: 'tpl-1',
    name: '潮汐流量',
    description: '模拟早高峰',
    shapeType: 'sine',
    controlPoints: [
        { x: 0, y: 10 },
        { x: 1800, y: 50 },
        { x: 3600, y: 10 },
    ],
    createdAt: '2026-08-20T00:00:00.000Z',
    updatedAt: '2026-08-20T00:00:00.000Z',
}

const renderLibrary = (onApply = vi.fn()) =>
    render(
        <MemoryRouter>
            <DndContext>
                <TemplateLibrary onApply={onApply} />
            </DndContext>
        </MemoryRouter>,
    )

describe('TemplateLibrary', () => {
    beforeEach(() => {
        useTrafficStore.setState({ templates: [template], overlays: [], mode: 'overview' })
    })

    it('渲染模板列表与数量', () => {
        renderLibrary()
        expect(screen.getByText('流量模板')).toBeInTheDocument()
        expect(screen.getByText('潮汐流量')).toBeInTheDocument()
    })

    it('点击绘制新模板切换为绘制模式', async () => {
        const user = userEvent.setup()
        renderLibrary()
        await user.click(screen.getByRole('button', { name: /绘制新模板/ }))
        expect(useTrafficStore.getState().mode).toBe('draw')
    })

    it('点击模板卡片调用 onApply', () => {
        const onApply = vi.fn()
        renderLibrary(onApply)
        fireEvent.click(screen.getByText('潮汐流量'))
        expect(onApply).toHaveBeenCalled()
    })

    it('删除模板后列表更新为空态', () => {
        renderLibrary()
        fireEvent.click(screen.getByTitle('删除模板'))
        expect(useTrafficStore.getState().templates).toHaveLength(0)
        expect(screen.getByText('模板库为空')).toBeInTheDocument()
    })

    it('模板库为空时展示查看填写指南链接', () => {
        useTrafficStore.setState({ templates: [] })
        renderLibrary()
        expect(screen.getByText('模板库为空')).toBeInTheDocument()
        expect(screen.getByRole('link', { name: /查看填写指南/ })).toBeInTheDocument()
    })
})