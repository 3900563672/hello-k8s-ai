import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { TooltipProvider } from '@/components/ui/tooltip'
import { AppSidebar } from '@/components/shared/Layout/AppSidebar'

vi.mock('@/components/shared/Layout/ClusterStatus', () => ({
    ClusterStatus: () => <div data-testid="cluster-status" />,
}))
vi.mock('@/components/shared/Layout/ExecutionControls', () => ({
    ExecutionControls: () => <div data-testid="execution-controls" />,
}))

const renderSidebar = () =>
    render(
        <MemoryRouter>
            <TooltipProvider>
                <AppSidebar />
            </TooltipProvider>
        </MemoryRouter>,
    )

describe('AppSidebar', () => {
    beforeEach(() => {
        localStorage.clear()
    })

    it('渲染四个导航入口与集群状态', () => {
        renderSidebar()
        expect(screen.getByRole('navigation', { name: '主导航' })).toBeInTheDocument()
        expect(screen.getByRole('link', { name: /状态总览/ })).toBeInTheDocument()
        expect(screen.getByRole('link', { name: /配置中心/ })).toBeInTheDocument()
        expect(screen.getByRole('link', { name: /流量布置/ })).toBeInTheDocument()
        expect(screen.getByRole('link', { name: /填写指南/ })).toBeInTheDocument()
        expect(screen.getByTestId('cluster-status')).toBeInTheDocument()
        expect(screen.getByTestId('execution-controls')).toBeInTheDocument()
    })

    it('点击展开按钮切换宽度并持久化到 localStorage', async () => {
        const user = userEvent.setup()
        renderSidebar()
        expect(localStorage.getItem('sidebar-expanded')).toBeNull()
        await user.click(screen.getByRole('button', { name: '展开导航' }))
        expect(localStorage.getItem('sidebar-expanded')).toBe('1')
        expect(screen.getByText('调度控制台')).toBeInTheDocument()
    })

    it('localStorage 为 1 时默认展开', () => {
        localStorage.setItem('sidebar-expanded', '1')
        renderSidebar()
        expect(screen.getByText('调度控制台')).toBeInTheDocument()
    })
})