import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, render, screen } from '@testing-library/react'
import { RouterProvider } from 'react-router-dom'
import { TooltipProvider } from '@/components/ui/tooltip'
import { router } from '@/app/router'

vi.mock('@/components/features/observatory/ObservatoryPage', () => ({
    ObservatoryPage: () => <div>观测台页面</div>,
}))
vi.mock('@/components/features/config/ConfigPage', () => ({
    ConfigPage: () => <div>配置中心页面</div>,
}))
vi.mock('@/components/features/traffic/TrafficPage', () => ({
    TrafficPage: () => <div>流量布置页面</div>,
}))
vi.mock('@/components/features/guide/GuidePage', () => ({
    GuidePage: () => <div>引导页面</div>,
}))
vi.mock('@/hooks/useWorkspaceContext', () => ({
    useWorkspaceCoordinator: () => null,
    useWorkspaceContext: () => ({
        mode: 'latest',
        effectiveAt: new Date(0).toISOString(),
        snapshotId: null,
        revision: 0,
        selectedSnapshot: null,
        cluster: { connectionStatus: 'connected', workers: [] },
        onlineWorkers: 0,
        totalWorkers: 0,
        executionMode: 'apply',
        executionPhase: 'idle',
        isHistorical: false,
        isWritable: true,
        canTest: false,
    }),
}))

vi.mock('@/hooks/useBackendSync', () => ({
    useBackendSync: () => null,
}))

vi.mock('@/components/features/aiops/AiChatWidget', () => ({
    AiChatWidget: () => <div data-testid="ai-chat-widget" />,
}))

vi.mock('@/components/shared/Layout/AppSidebar', () => ({
    AppSidebar: () => <nav data-testid="app-sidebar">导航</nav>,
}))

vi.mock('@/components/shared/TimeTravelBar/TimeTravelBar', () => ({
    TimeTravelBar: () => <div data-testid="time-travel-bar" />,
}))



const navigate = (to: string) =>
    act(async () => {
        router.navigate(to)
    })

describe('app router', () => {
    afterEach(() => {
        cleanup()
    })

    it('根路由挂载 MainLayout 并注册全部子路由', () => {
        const root = router.routes[0]
        expect(root.path).toBe('/')
        expect(root.children?.length).toBe(8)
        const paths = root.children!.map((child) => child.path)
        expect(paths).toEqual(
            expect.arrayContaining(['observatory', 'config', 'traffic', 'guide']),
        )
        expect(paths).toContain('*')
    })

    it('index 路由重定向到 observatory', () => {
        const root = router.routes[0]
        const index = root.children!.find((child) => child.index)
        expect(index).toBeDefined()
        expect(JSON.stringify(index!.element)).toContain('/observatory')
    })

    it('trace/monitor 路由重定向到 observatory', () => {
        const root = router.routes[0]
        const trace = root.children!.find((child) => child.path === 'trace')
        const monitor = root.children!.find((child) => child.path === 'monitor')
        expect(JSON.stringify(trace!.element)).toContain('/observatory')
        expect(JSON.stringify(monitor!.element)).toContain('/observatory')
    })

    it('根路由配置错误边界与 404 兜底', () => {
        const root = router.routes[0]
        expect(root.errorElement).toBeTruthy()
        const wildcard = root.children!.find((child) => child.path === '*')
        expect(wildcard).toBeDefined()
    })

    it('渲染懒加载页面（config）', async () => {
        render(
            <TooltipProvider>
                <RouterProvider router={router} />
            </TooltipProvider>,
        )
        await navigate('/config')
        expect(await screen.findByText('配置中心页面')).toBeInTheDocument()
    })

    it('trace 重定向到 observatory 并渲染懒加载页面', async () => {
        render(
            <TooltipProvider>
                <RouterProvider router={router} />
            </TooltipProvider>,
        )
        await navigate('/trace')
        expect(await screen.findByText('观测台页面')).toBeInTheDocument()
    })

    it('未知路径渲染 404 页', async () => {
        render(
            <TooltipProvider>
                <RouterProvider router={router} />
            </TooltipProvider>,
        )
        await navigate('/no-such-page')
        expect(await screen.findByText('没有这个工作区页面')).toBeInTheDocument()
    })
})