import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

describe('PageLoader', () => {
    it('展示加载状态与 aria 角色', async () => {
        vi.resetModules()
        const { PageLoader } = await import('@/components/shared/feedback/RouteFallbacks')
        render(<PageLoader />)
        expect(screen.getByRole('status')).toBeInTheDocument()
        expect(screen.getByText('正在载入工作区')).toBeInTheDocument()
    })
})

describe('NotFoundPage', () => {
    it('展示 404 与返回配置中心入口', async () => {
        vi.resetModules()
        const { NotFoundPage } = await import('@/components/shared/feedback/RouteFallbacks')
        render(
            <MemoryRouter>
                <NotFoundPage />
            </MemoryRouter>,
        )
        expect(screen.getByText('404')).toBeInTheDocument()
        expect(screen.getByText('没有这个工作区页面')).toBeInTheDocument()
        expect(screen.getByRole('link', { name: /返回配置中心/ })).toBeInTheDocument()
    })
})

describe('RouteErrorBoundary', () => {
    beforeEach(() => {
        vi.resetModules()
    })

    it('路由错误（404）时展示状态码描述', async () => {
        vi.doMock('react-router-dom', async (importOriginal) => {
            const actual = await importOriginal<typeof import('react-router-dom')>()
            return {
                ...actual,
                useRouteError: () => ({
                    status: 404,
                    statusText: 'Not Found',
                    internal: false,
                    data: undefined,
                }),
            }
        })
        const { RouteErrorBoundary } = await import('@/components/shared/feedback/RouteFallbacks')
        render(
            <MemoryRouter>
                <RouteErrorBoundary />
            </MemoryRouter>,
        )
        expect(screen.getByText('工作区无法打开')).toBeInTheDocument()
        expect(screen.getByText(/404 Not Found/)).toBeInTheDocument()
    })

    it('Error 实例时展示错误消息', async () => {
        vi.doMock('react-router-dom', async (importOriginal) => {
            const actual = await importOriginal<typeof import('react-router-dom')>()
            return {
                ...actual,
                useRouteError: () => new Error('boom'),
            }
        })
        const { RouteErrorBoundary } = await import('@/components/shared/feedback/RouteFallbacks')
        render(
            <MemoryRouter>
                <RouteErrorBoundary />
            </MemoryRouter>,
        )
        expect(screen.getByText('boom')).toBeInTheDocument()
    })
})