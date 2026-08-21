import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TenantTable } from '@/components/features/config/tables/TenantTable'
import type { Tenant } from '@/types/config.types'

const data: Tenant[] = [
    {
        name: 'tenant-a',
        displayName: '租户A',
        priority: 'P1',
        qps: 100,
        ttftThresholdMs: 2000,
        queueThreshold: 50,
        ttftScaleDownThresholdMs: 1000,
        queueScaleDownThreshold: 20,
    },
    {
        name: 'tenant-b',
        displayName: '租户B',
        priority: 'P5',
        qps: 30,
        ttftThresholdMs: 2000,
        queueThreshold: 50,
        ttftScaleDownThresholdMs: 1000,
        queueScaleDownThreshold: 20,
    },
]

const renderTable = (props: Partial<Parameters<typeof TenantTable>[0]> = {}) =>
    render(
        <TenantTable
            data={data}
            onSelect={vi.fn()}
            onDelete={vi.fn()}
            selectedIds={[]}
            onSelectionChange={vi.fn()}
            {...props}
        />,
    )

describe('TenantTable', () => {
    it('渲染租户、优先级与基准 QPS 列', () => {
        renderTable()
        expect(screen.getByText('租户A')).toBeInTheDocument()
        expect(screen.getByText('P1')).toBeInTheDocument()
        expect(screen.getByText('P5')).toBeInTheDocument()
        expect(screen.getByText('100')).toBeInTheDocument()
        expect(screen.getByText('30')).toBeInTheDocument()
    })

    it('点击行触发 onSelect', async () => {
        const user = userEvent.setup()
        const onSelect = vi.fn()
        renderTable({ onSelect })
        await user.click(screen.getByText('租户B'))
        expect(onSelect).toHaveBeenCalledWith(data[1])
    })
})