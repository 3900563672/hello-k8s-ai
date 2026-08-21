import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CreateDialog } from '@/components/shared/dialogs/CreateDialog'

const Harness = ({ initial = '', error = '', onConfirm }: {
    initial?: string
    error?: string
    onConfirm?: () => void
}) => {
    const [value, setValue] = useState(initial)
    return (
        <CreateDialog
            open
            onOpenChange={vi.fn()}
            type="tenant"
            value={value}
            onValueChange={setValue}
            identifierPreview="tenant-aiops-prod"
            error={error}
            onConfirm={onConfirm ?? vi.fn()}
        />
    )
}

describe('CreateDialog', () => {
    it('展示新建标题与系统标识预览', () => {
        render(<Harness initial="租户A" />)
        expect(screen.getByText('新建租户')).toBeInTheDocument()
        expect(screen.getByText('tenant-aiops-prod')).toBeInTheDocument()
    })

    it('输入后回车触发确认', async () => {
        const user = userEvent.setup()
        const onConfirm = vi.fn()
        render(<Harness onConfirm={onConfirm} />)
        const input = screen.getByLabelText('显示名称')
        await user.type(input, '新租户{enter}')
        expect(onConfirm).toHaveBeenCalled()
    })

    it('空名称回车不触发确认', async () => {
        const user = userEvent.setup()
        const onConfirm = vi.fn()
        render(<Harness onConfirm={onConfirm} />)
        await user.type(screen.getByLabelText('显示名称'), '{enter}')
        expect(onConfirm).not.toHaveBeenCalled()
    })

    it('展示错误信息', () => {
        render(<Harness error="标识已存在" />)
        expect(screen.getByText('标识已存在')).toBeInTheDocument()
    })
})