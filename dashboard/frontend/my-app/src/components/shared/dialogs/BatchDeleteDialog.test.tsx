import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BatchDeleteDialog } from '@/components/shared/dialogs/BatchDeleteDialog'

describe('BatchDeleteDialog', () => {
    const props = {
        open: true,
        onOpenChange: vi.fn(),
        typeLabel: '节点',
        count: 3,
        onConfirm: vi.fn(),
    }

    it('展示删除数量与确认回调', async () => {
        const user = userEvent.setup()
        render(<BatchDeleteDialog {...props} />)
        expect(screen.getByText('删除 3 个节点？')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: '确认删除' }))
        expect(props.onConfirm).toHaveBeenCalled()
    })

    it('count 为 0 时确认按钮禁用', () => {
        render(<BatchDeleteDialog {...props} count={0} />)
        expect(screen.getByRole('button', { name: '确认删除' })).toBeDisabled()
    })

    it('pending 时展示正在删除并禁用取消', () => {
        render(<BatchDeleteDialog {...props} pending />)
        expect(screen.getByRole('button', { name: '正在删除' })).toBeDisabled()
        expect(screen.getByRole('button', { name: '取消' })).toBeDisabled()
    })

    it('展示错误信息', () => {
        render(<BatchDeleteDialog {...props} error="删除失败：后端拒绝" />)
        expect(screen.getByText('删除失败：后端拒绝')).toBeInTheDocument()
    })
})