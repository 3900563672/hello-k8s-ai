import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { OrchestratorTable } from '@/components/features/config/tables/OrchestratorTable'
import type { Orchestrator } from '@/types/config.types'

const data: Orchestrator[] = [
    {
        name: 'orchestrator-a',
        displayName: '租户A',
        tenantRef: { name: 'tenant-a' },
        scaleUpCooldownSeconds: 60,
        scaleDownCooldownSeconds: 120,
        allowScaleToZero: true,
        minReplicas: 1,
        maxReplicas: 0,
        maxScaleUpBatch: 0,
    },
]

const renderTable = (props: Partial<Parameters<typeof OrchestratorTable>[0]> = {}) =>
    render(
        <OrchestratorTable
            data={data}
            onSelect={vi.fn()}
            onDelete={vi.fn()}
            selectedIds={[]}
            onSelectionChange={vi.fn()}
            {...props}
        />,
    )

describe('OrchestratorTable', () => {
    it('渲染关联租户、副本范围与冷却列', () => {
        renderTable()
        expect(screen.getByText('租户A')).toBeInTheDocument()
        expect(screen.getByText(/1 – ∞/)).toBeInTheDocument()
        expect(screen.getByText('默认 10')).toBeInTheDocument()
        expect(screen.getByText('60s')).toBeInTheDocument()
        expect(screen.getByText('120s')).toBeInTheDocument()
        expect(screen.getByText('允许')).toBeInTheDocument()
    })

    it('点击行触发 onSelect', async () => {
        const user = userEvent.setup()
        const onSelect = vi.fn()
        renderTable({ onSelect })
        await user.click(screen.getByText('租户A'))
        expect(onSelect).toHaveBeenCalledWith(data[0])
    })
})