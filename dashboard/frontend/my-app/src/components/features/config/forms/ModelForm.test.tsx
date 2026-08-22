import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ModelForm } from '@/components/features/config/forms/ModelForm'
import { useTemplateStore } from '@/stores/templateSlice'
import type { ModelFormValues } from '@/lib/validations/model.schema'

const defaultValues: ModelFormValues = {
    displayName: '生产模型',
    gpuUnits: 16,
    maxConcurrency: 32,
    absoluteScore: 100,
    coldStartMs: 1500,
    performance: {
        prefillBaseMs: 50,
        prefillPerTokenUs: 500,
        decodePerTokenMs: 20,
    },
}

const renderForm = (onSubmit = vi.fn()) =>
    render(<ModelForm defaultValues={defaultValues} onSubmit={onSubmit} />)

describe('ModelForm', () => {
    beforeEach(() => {
        useTemplateStore.setState({ modelTemplates: [] })
    })

    it('渲染基础配置区块与保存按钮，回填默认值', () => {
        renderForm()
        expect(screen.getByText('基础配置')).toBeInTheDocument()
        expect(screen.getByText('性能画像')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /保存模型/ })).toBeInTheDocument()
        expect(screen.getByLabelText('显示名称')).toHaveValue('生产模型')
        expect(screen.getAllByRole('spinbutton')[0]).toHaveValue(16)
    })

    it('修改字段后提交携带表单值', async () => {
        const user = userEvent.setup()
        const onSubmit = vi.fn()
        renderForm(onSubmit)
        await user.clear(screen.getByLabelText('显示名称'))
        await user.type(screen.getByLabelText('显示名称'), '新模型')
        await user.click(screen.getByRole('button', { name: /保存模型/ }))
        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ displayName: '新模型' }))
    })

    it('空名称提交展示校验错误', async () => {
        const user = userEvent.setup()
        const onSubmit = vi.fn()
        renderForm(onSubmit)
        await user.clear(screen.getByLabelText('显示名称'))
        await user.click(screen.getByRole('button', { name: /保存模型/ }))
        expect(await screen.findByText('模型名称不能为空')).toBeInTheDocument()
        expect(onSubmit).not.toHaveBeenCalled()
    })

    it('性能画像折叠区可展开并提交高级参数', async () => {
        const user = userEvent.setup()
        const onSubmit = vi.fn()
        renderForm(onSubmit)
        await user.click(screen.getByRole('button', { name: /展开/ }))
        const spinbuttons = screen.getAllByRole('spinbutton')
        expect(spinbuttons.length).toBe(7)
        await user.clear(spinbuttons[4])
        await user.type(spinbuttons[4], '120')
        await user.click(screen.getByRole('button', { name: /保存模型/ }))
        expect(onSubmit).toHaveBeenCalledWith(
            expect.objectContaining({ performance: expect.objectContaining({ prefillBaseMs: 120 }) }),
        )
    })

    it('保存模板后模板库出现新模板', async () => {
        const user = userEvent.setup()
        renderForm()
        await user.click(screen.getByRole('button', { name: /存为模板/ }))
        await user.type(screen.getByLabelText('模板名称'), '生产模板')
        await user.click(screen.getByRole('button', { name: '保存模板' }))
        await user.click(screen.getByRole('button', { name: /模板库/ }))
        expect(await screen.findByText('生产模板')).toBeInTheDocument()
    })
})