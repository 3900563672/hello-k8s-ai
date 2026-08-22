import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { MainLayout } from '@/components/shared/Layout/MainLayout'

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

describe('MainLayout', () => {
    it('渲染侧边栏、时间条、AI 浮窗与跳转锚点', () => {
        render(
            <MemoryRouter>
                <Routes>
                    <Route element={<MainLayout />}>
                        <Route path="/" element={<div data-testid="page-outlet">页面内容</div>} />
                    </Route>
                </Routes>
            </MemoryRouter>,
        )
        expect(screen.getByTestId('app-sidebar')).toBeInTheDocument()
        expect(screen.getByTestId('time-travel-bar')).toBeInTheDocument()
        expect(screen.getByTestId('ai-chat-widget')).toBeInTheDocument()
        expect(screen.getByTestId('page-outlet')).toBeInTheDocument()
        expect(screen.getByText('跳到主内容')).toBeInTheDocument()
    })
})