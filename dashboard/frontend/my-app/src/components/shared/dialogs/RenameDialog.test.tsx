import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { RenameDialog } from '@/components/shared/dialogs/RenameDialog'

const Harness = ({ initial = '生产节点', pending = false, error = '', onConfirm }: {
    initial?: string
    pending?: boolean
    error?: string
    onConfirm?: () => void
}) => {
    const [value, setValue] = useState(initial)
    return (
        <RenameDialog
            open
            onOpenChange={vi.fn()}
            typeLabel="节点"
            resourceId="node-1"
            value={value}
            onValueChange={setValue}
            pending={pending}
            error={error}
            onConfirm={onConfirm ?? vi.fn()}
        />
    )
}

describe('RenameDialog', () => {
    it('展示标题与当前值，修改后点击确认回调', async () => {
        const user = userEvent.setup()
        const onConfirm = vi.fn()
        render(<Harness onConfirm={onConfirm} />)
        expect(screen.getByText('重命名节点')).toBeInTheDocument()
        const input = screen.getByLabelText('显示名称')
        expect(input).toHaveValue('生产节点')

        await user.clear(input)
        await user.type(input, '新节点名')
        expect(input).toHaveValue('新节点名')
        await user.click(screen.getByRole('button', { name: '保存名称' }))
        expect(onConfirm).toHaveBeenCalled()
    })

    it('pending 时按钮禁用并保持保存文案', () => {
        render(<Harness pending />)
        expect(screen.getByRole('button', { name: /保存名称/ })).toBeDisabled()
        expect(screen.getByRole('button', { name: '取消' })).toBeDisabled()
    })

    it('展示错误信息', () => {
        render(<Harness error="名称已存在" />)
        expect(screen.getByText('名称已存在')).toBeInTheDocument()
    })
})