import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { NodeForm } from '@/components/features/config/forms/NodeForm'
import { useTemplateStore } from '@/stores/templateSlice'
import type { NodeFormValues } from '@/lib/validations/node.schema'

const defaultValues: NodeFormValues = {
    displayName: '生产节点',
    gpu: 80,
    maxConcurrency: 128,
}

const renderForm = (onSubmit = vi.fn()) =>
    render(<NodeForm defaultValues={defaultValues} onSubmit={onSubmit} />)

describe('NodeForm', () => {
    beforeEach(() => {
        useTemplateStore.setState({ nodeTemplates: [] })
    })

    it('渲染节点容量区块、单并发显存与保存按钮', () => {
        renderForm()
        expect(screen.getByText('节点容量')).toBeInTheDocument()
        expect(screen.getByText('单并发平均显存')).toBeInTheDocument()
        expect(screen.getByText('0.63 G')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /保存节点/ })).toBeInTheDocument()
        expect(screen.getByLabelText('显示名称')).toHaveValue('生产节点')
    })

    it('修改字段后提交携带表单值', async () => {
        const user = userEvent.setup()
        const onSubmit = vi.fn()
        renderForm(onSubmit)
        await user.clear(screen.getByLabelText('显示名称'))
        await user.type(screen.getByLabelText('显示名称'), '新节点')
        await user.click(screen.getByRole('button', { name: /保存节点/ }))
        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ displayName: '新节点' }))
    })

    it('空名称提交展示校验错误', async () => {
        const user = userEvent.setup()
        const onSubmit = vi.fn()
        renderForm(onSubmit)
        await user.clear(screen.getByLabelText('显示名称'))
        await user.click(screen.getByRole('button', { name: /保存节点/ }))
        expect(await screen.findByText('节点名称不能为空')).toBeInTheDocument()
        expect(onSubmit).not.toHaveBeenCalled()
    })

    it('总显存修改后单并发显存实时重算', async () => {
        const user = userEvent.setup()
        renderForm()
        await user.clear(screen.getAllByRole('spinbutton')[0])
        await user.type(screen.getAllByRole('spinbutton')[0], '64')
        expect(screen.getByText('0.5 G')).toBeInTheDocument()
    })
})