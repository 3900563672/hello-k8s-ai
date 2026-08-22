import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TenantForm } from '@/components/features/config/forms/TenantForm'
import { useTemplateStore } from '@/stores/templateSlice'
import type { TenantFormValues } from '@/lib/validations/tenant.schema'

const defaultValues: TenantFormValues = {
    displayName: '生产租户',
    priority: 'P3',
    qps: 100,
    ttftThresholdMs: 2000,
    queueThreshold: 50,
    ttftScaleDownThresholdMs: 1000,
    queueScaleDownThreshold: 20,
}

const renderForm = (onSubmit = vi.fn()) =>
    render(<TenantForm defaultValues={defaultValues} onSubmit={onSubmit} />)

describe('TenantForm', () => {
    beforeEach(() => {
        useTemplateStore.setState({ tenantTemplates: [], modelTemplates: [], nodeTemplates: [], orchestratorTemplates: [] })
    })

    it('渲染身份与流量、弹性阈值区块与保存按钮', () => {
        renderForm()
        expect(screen.getByText('身份与流量')).toBeInTheDocument()
        expect(screen.getByText('弹性阈值')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /保存租户/ })).toBeInTheDocument()
        expect(screen.getByLabelText('显示名称')).toHaveValue('生产租户')
    })

    it('修改字段后提交携带表单值', async () => {
        const user = userEvent.setup()
        const onSubmit = vi.fn()
        renderForm(onSubmit)
        await user.clear(screen.getByLabelText('显示名称'))
        await user.type(screen.getByLabelText('显示名称'), '新租户')
        await user.click(screen.getByRole('button', { name: /保存租户/ }))
        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ displayName: '新租户' }))
    })

    it('空名称提交展示校验错误', async () => {
        const user = userEvent.setup()
        const onSubmit = vi.fn()
        renderForm(onSubmit)
        await user.clear(screen.getByLabelText('显示名称'))
        await user.click(screen.getByRole('button', { name: /保存租户/ }))
        expect(await screen.findByText('租户名称不能为空')).toBeInTheDocument()
        expect(onSubmit).not.toHaveBeenCalled()
    })

    it('缩容阈值不低于扩容阈值时展示错误', async () => {
        const user = userEvent.setup()
        const onSubmit = vi.fn()
        renderForm(onSubmit)
        const scaleDownInput = screen.getAllByRole('spinbutton')[3]
        await user.clear(scaleDownInput)
        await user.type(scaleDownInput, '3000')
        await user.click(screen.getByRole('button', { name: /保存租户/ }))
        expect(await screen.findByText('缩容阈值必须小于扩容阈值')).toBeInTheDocument()
        expect(onSubmit).not.toHaveBeenCalled()
    })
})