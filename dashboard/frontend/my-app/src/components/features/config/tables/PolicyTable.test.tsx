import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { PolicyTable } from '@/components/features/config/tables/PolicyTable'
import type { Policy } from '@/types/config.types'

const data: Policy[] = [
    { name: 'policy-a', displayName: '租户A-模型A', kind: 'tenantModel', tenantRef: { name: 'tenant-a' }, modelRef: { name: 'model-a' }, effect: 'Allow' },
    { name: 'policy-b', displayName: '模型A-节点A', kind: 'modelNode', modelRef: { name: 'model-a' }, nodeRef: { name: 'node-a' }, effect: 'Deny' },
]

const renderTable = (props: Partial<Parameters<typeof PolicyTable>[0]> = {}) =>
    render(
        <PolicyTable
            data={data}
            onSelect={vi.fn()}
            onDelete={vi.fn()}
            selectedIds={[]}
            onSelectionChange={vi.fn()}
            {...props}
        />,
    )

describe('PolicyTable', () => {
    it('渲染引用关系、类型与效果列', () => {
        renderTable()
        expect(screen.getByText('租户A-模型A')).toBeInTheDocument()
        expect(screen.getByText('租户-模型')).toBeInTheDocument()
        expect(screen.getByText('模型-节点')).toBeInTheDocument()
        expect(screen.getByText('允许')).toBeInTheDocument()
        expect(screen.getByText('禁止')).toBeInTheDocument()
    })

    it('点击行触发 onSelect', async () => {
        const user = userEvent.setup()
        const onSelect = vi.fn()
        renderTable({ onSelect })
        await user.click(screen.getByText('模型A-节点A'))
        expect(onSelect).toHaveBeenCalledWith(data[1])
    })
})