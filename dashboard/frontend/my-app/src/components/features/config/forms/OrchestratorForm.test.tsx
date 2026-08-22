import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { OrchestratorForm } from '@/components/features/config/forms/OrchestratorForm'
import { useTemplateStore } from '@/stores/templateSlice'
import type { OrchestratorFormValues } from '@/lib/validations/orchestrator.schema'

vi.mock('@/api/queries/configQueries', () => ({
    useTenants: () => ({ data: [{ name: 'tenant-a', displayName: '租户A' }] }),
}))

const defaultValues: OrchestratorFormValues = {
    tenantName: 'tenant-a',
    scaleUpCooldownSeconds: 60,
    scaleDownCooldownSeconds: 120,
    allowScaleToZero: false,
    minReplicas: 1,
    maxReplicas: 10,
    maxScaleUpBatch: 2,
}

const renderForm = (onSubmit = vi.fn()) =>
    render(<OrchestratorForm defaultValues={defaultValues} onSubmit={onSubmit} />)

describe('OrchestratorForm', () => {
    beforeEach(() => {
        useTemplateStore.setState({ orchestratorTemplates: [] })
    })

    it('渲染关联与副本策略区块、关联租户下拉回填', () => {
        renderForm()
        expect(screen.getByText('关联与冷却')).toBeInTheDocument()
        expect(screen.getByText('副本策略')).toBeInTheDocument()
        expect(screen.getByText('允许缩容到零')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /保存编排策略/ })).toBeInTheDocument()
        expect(screen.getByText('租户A')).toBeInTheDocument()
    })

    it('修改冷却时间后提交携带表单值', async () => {
        const user = userEvent.setup()
        const onSubmit = vi.fn()
        renderForm(onSubmit)
        await user.clear(screen.getAllByRole('spinbutton')[0])
        await user.type(screen.getAllByRole('spinbutton')[0], '90')
        await user.click(screen.getByRole('button', { name: /保存编排策略/ }))
        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ scaleUpCooldownSeconds: 90 }))
    })

    it('最小副本数大于最大副本数时展示校验错误', async () => {
        const user = userEvent.setup()
        const onSubmit = vi.fn()
        renderForm(onSubmit)
        await user.clear(screen.getAllByRole('spinbutton')[2])
        await user.type(screen.getAllByRole('spinbutton')[2], '20')
        await user.click(screen.getByRole('button', { name: /保存编排策略/ }))
        expect(await screen.findByText('最小副本数不能大于最大副本数')).toBeInTheDocument()
        expect(onSubmit).not.toHaveBeenCalled()
    })

    it('切换允许缩容到零后提交为 true', async () => {
        const user = userEvent.setup()
        const onSubmit = vi.fn()
        renderForm(onSubmit)
        await user.click(screen.getByRole('checkbox', { name: /允许缩容到零/ }))
        await user.click(screen.getByRole('button', { name: /保存编排策略/ }))
        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ allowScaleToZero: true }))
    })
})