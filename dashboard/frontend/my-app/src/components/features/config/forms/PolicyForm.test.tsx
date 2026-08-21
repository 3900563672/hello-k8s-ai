import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { PolicyForm } from '@/components/features/config/forms/PolicyForm'
import type { PolicyFormValues } from '@/lib/validations/policy.schema'

vi.mock('@/api/queries/configQueries', () => ({
    useTenants: () => ({ data: [{ name: 'tenant-a', displayName: '租户A' }] }),
    useModels: () => ({ data: [{ name: 'model-a', displayName: '模型A' }] }),
    useNodes: () => ({ data: [{ name: 'node-a', displayName: '节点A' }] }),
}))

const baseValues: PolicyFormValues = {
    kind: 'tenantModel',
    tenantName: 'tenant-a',
    modelName: 'model-a',
    nodeName: '',
    effect: 'Allow',
}

const renderForm = (values: PolicyFormValues, onSubmit = vi.fn()) =>
    render(<PolicyForm defaultValues={values} onSubmit={onSubmit} />)

const submitForm = (container: HTMLElement) => {
    fireEvent.submit(container.querySelector('form')!)
}

describe('PolicyForm', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    it('tenantModel 类型渲染租户与模型选择', () => {
        renderForm(baseValues)
        expect(screen.getByText('租户-模型策略')).toBeInTheDocument()
        expect(screen.getByText('关联租户')).toBeInTheDocument()
        expect(screen.getByText('关联模型')).toBeInTheDocument()
        expect(screen.queryByText('关联节点')).not.toBeInTheDocument()
        expect(screen.getByRole('button', { name: /保存策略/ })).toBeInTheDocument()
    })

    it('tenantNode 类型渲染租户与节点选择', () => {
        renderForm({ ...baseValues, kind: 'tenantNode', modelName: '', nodeName: 'node-a' })
        expect(screen.getByText('租户-节点策略')).toBeInTheDocument()
        expect(screen.getByText('关联节点')).toBeInTheDocument()
        expect(screen.queryByText('关联模型')).not.toBeInTheDocument()
    })

    it('modelNode 类型渲染模型与节点选择', () => {
        renderForm({ ...baseValues, kind: 'modelNode', tenantName: '', modelName: 'model-a', nodeName: 'node-a' })
        expect(screen.getByText('模型-节点策略')).toBeInTheDocument()
        expect(screen.queryByText('关联租户')).not.toBeInTheDocument()
        expect(screen.getByText('关联模型')).toBeInTheDocument()
    })

    it('缺少租户时提交展示校验错误', async () => {
        const onSubmit = vi.fn()
        const { container } = renderForm({ ...baseValues, tenantName: '' }, onSubmit)
        submitForm(container)
        expect(await screen.findByText('请选择租户')).toBeInTheDocument()
        expect(onSubmit).not.toHaveBeenCalled()
    })

    it('默认效果为 Deny 时提交携带该值', async () => {
        const onSubmit = vi.fn()
        const { container } = renderForm({ ...baseValues, effect: 'Deny' }, onSubmit)
        expect(screen.getByText('Deny')).toBeInTheDocument()
        submitForm(container)
        expect(await screen.findByRole('button', { name: /保存策略/ })).toBeInTheDocument()
        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ effect: 'Deny' }))
    })
})