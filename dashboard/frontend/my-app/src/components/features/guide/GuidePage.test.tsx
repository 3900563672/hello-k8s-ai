import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { GuidePage } from '@/components/features/guide/GuidePage'

describe('GuidePage', () => {
    it('渲染配置字段指南与预置模板示例', () => {
        render(
            <MemoryRouter>
                <GuidePage />
            </MemoryRouter>,
        )
        expect(screen.getAllByText('标识符生成规则').length).toBeGreaterThan(0)
        expect(screen.getAllByText('模型').length).toBeGreaterThan(0)
        expect(screen.getAllByText('租户').length).toBeGreaterThan(0)
        expect(screen.getAllByText('节点').length).toBeGreaterThan(0)
        expect(screen.getAllByText('编排策略').length).toBeGreaterThan(0)
        expect(screen.getAllByText('预置模板示例').length).toBeGreaterThan(0)
    })

    it('提供返回配置中心的链接', () => {
        render(
            <MemoryRouter>
                <GuidePage />
            </MemoryRouter>,
        )
        const link = screen.getByRole('link', { name: /返回配置中心/ })
        expect(link).toHaveAttribute('href', '/config')
    })
})