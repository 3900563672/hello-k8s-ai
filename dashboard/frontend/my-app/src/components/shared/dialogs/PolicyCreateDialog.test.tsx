import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { PolicyCreateDialog } from '@/components/shared/dialogs/PolicyCreateDialog'
import type { Model, Node, Tenant } from '@/types/config.types'

const tenants: Tenant[] = [
    { name: 'tenant-a', displayName: '租户A', priority: 'P1', qps: 100, ttftThresholdMs: 2000, queueThreshold: 50, ttftScaleDownThresholdMs: 1000, queueScaleDownThreshold: 20 },
]
const models: Model[] = [
    { name: 'model-a', displayName: '模型A', gpuUnits: 8, maxConcurrency: 16, absoluteScore: 75, coldStartMs: 800, performance: { prefillBaseMs: 50, prefillPerTokenUs: 500, decodePerTokenMs: 20 } },
]
const nodes: Node[] = [{ name: 'node-a', displayName: '节点A', gpu: 80, maxConcurrency: 128 }]

const renderDialog = (props: Partial<Parameters<typeof PolicyCreateDialog>[0]> = {}) =>
    render(
        <PolicyCreateDialog
            open
            onOpenChange={vi.fn()}
            kind="tenantModel"
            onKindChange={vi.fn()}
            tenantName=""
            onTenantNameChange={vi.fn()}
            modelName=""
            onModelNameChange={vi.fn()}
            nodeName=""
            onNodeNameChange={vi.fn()}
            effect="Allow"
            onEffectChange={vi.fn()}
            identifierPreview=""
            tenants={tenants}
            models={models}
            nodes={nodes}
            onConfirm={vi.fn()}
            {...props}
        />,
    )

describe('PolicyCreateDialog', () => {
    it('tenantModel 类型展示租户与模型选择，未选齐时确认禁用', () => {
        renderDialog()
        expect(screen.getByText('新建策略')).toBeInTheDocument()
        expect(screen.getByText('策略类型')).toBeInTheDocument()
        expect(screen.getByText('租户')).toBeInTheDocument()
        expect(screen.getByText('模型')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /创建策略/ })).toBeDisabled()
    })

    it('选齐引用后确认可用并触发 onConfirm', async () => {
        const user = userEvent.setup()
        const onConfirm = vi.fn()
        renderDialog({ tenantName: 'tenant-a', modelName: 'model-a', onConfirm })
        expect(screen.getByRole('button', { name: /创建策略/ })).toBeEnabled()
        await user.click(screen.getByRole('button', { name: /创建策略/ }))
        expect(onConfirm).toHaveBeenCalled()
    })

    it('modelNode 类型展示模型与节点选择', () => {
        renderDialog({ kind: 'modelNode' })
        expect(screen.getByText('模型')).toBeInTheDocument()
        expect(screen.getByText('节点')).toBeInTheDocument()
    })

    it('pending 时确认按钮显示正在创建并禁用', () => {
        renderDialog({ tenantName: 'tenant-a', modelName: 'model-a', pending: true })
        expect(screen.getByRole('button', { name: /正在创建/ })).toBeDisabled()
    })

    it('展示错误信息', () => {
        renderDialog({ error: '创建失败：重名' })
        expect(screen.getByText('创建失败：重名')).toBeInTheDocument()
    })
})