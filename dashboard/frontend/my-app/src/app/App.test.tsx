import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import App from '@/app/App'

vi.mock('react-router-dom', () => ({
    RouterProvider: () => <div data-testid="router-provider" />,
}))
vi.mock('@/app/router', () => ({ router: {} }))
vi.mock('@tanstack/react-query-devtools', () => ({
    ReactQueryDevtools: () => null,
}))

describe('App', () => {
    it('渲染 QueryClient/Tooltip/RouterProvider 应用外壳', () => {
        render(<App />)
        expect(screen.getByTestId('router-provider')).toBeInTheDocument()
    })
})
