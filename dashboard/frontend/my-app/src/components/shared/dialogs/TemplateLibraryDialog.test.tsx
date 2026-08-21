import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TemplateLibraryDialog } from '@/components/shared/dialogs/TemplateLibraryDialog'

const templates = [
    { id: 'tpl-1', name: '高性能配置', preset: true, createdAt: '2026-01-01T00:00:00.000Z', data: { displayName: 'x' } },
    { id: 'tpl-2', name: '省显存配置', createdAt: '2026-02-01T00:00:00.000Z', data: { displayName: 'y' } },
]

const renderDialog = (props: Partial<Parameters<typeof TemplateLibraryDialog<{ displayName: string }>>[0]> = {}) =>
    render(
        <TemplateLibraryDialog
            open
            onOpenChange={vi.fn()}
            templates={templates}
            typeLabel="模型"
            onLoad={vi.fn()}
            onDelete={vi.fn()}
            getPreview={(data) => [{ key: '名称', value: data.displayName }]}
            {...props}
        />,
    )

describe('TemplateLibraryDialog', () => {
    it('渲染模板卡片与预置标记', () => {
        renderDialog()
        expect(screen.getByText('模板库 — 模型')).toBeInTheDocument()
        expect(screen.getByText('高性能配置')).toBeInTheDocument()
        expect(screen.getByText('预置')).toBeInTheDocument()
        expect(screen.getByText('省显存配置')).toBeInTheDocument()
    })

    it('加载模板与删除触发回调', async () => {
        const user = userEvent.setup()
        const onLoad = vi.fn()
        const onDelete = vi.fn()
        renderDialog({ onLoad, onDelete })
        const loadButtons = screen.getAllByRole('button', { name: '加载模板' })
        await user.click(loadButtons[0])
        expect(onLoad).toHaveBeenCalledWith(templates[0])
        const deleteButtons = screen.getAllByRole('button', { name: '删除' })
        await user.click(deleteButtons[1])
        expect(onDelete).toHaveBeenCalledWith('tpl-2')
    })

    it('pickMode 隐藏删除按钮并改用“使用此模板”', () => {
        renderDialog({ pickMode: true })
        expect(screen.queryByRole('button', { name: '删除' })).not.toBeInTheDocument()
        expect(screen.getAllByRole('button', { name: '使用此模板' }).length).toBe(2)
    })

    it('无模板时展示空态', () => {
        renderDialog({ templates: [] })
        expect(screen.getByText('暂无已保存的模型模板')).toBeInTheDocument()
    })
})