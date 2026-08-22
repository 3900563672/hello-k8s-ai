import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { NodeTable } from '@/components/features/config/tables/NodeTable'
import type { Node } from '@/types/config.types'

const data: Node[] = [
    {
        name: 'node-a',
        displayName: '节点A',
        gpu: 80,
        maxConcurrency: 128,
        status: { usedGPU: 60 },
    },
    {
        name: 'node-b',
        displayName: '节点B',
        gpu: 80,
        maxConcurrency: 64,
    },
]

const renderTable = (props: Partial<Parameters<typeof NodeTable>[0]> = {}) =>
    render(
        <NodeTable
            data={data}
            onSelect={vi.fn()}
            onDelete={vi.fn()}
            selectedIds={[]}
            onSelectionChange={vi.fn()}
            {...props}
        />,
    )

describe('NodeTable', () => {
    it('渲染节点列表、GPU 用量与并发列', () => {
        renderTable()
        expect(screen.getByText('节点A')).toBeInTheDocument()
        expect(screen.getByText('60')).toBeInTheDocument()
        expect(screen.getByText(/80 G/)).toBeInTheDocument()
        expect(screen.getByText('128')).toBeInTheDocument()
    })

    it('无 usedGPU 状态时展示占位符', () => {
        renderTable()
        expect(screen.getAllByText('—').length).toBeGreaterThan(0)
    })

    it('点击行触发 onSelect', async () => {
        const user = userEvent.setup()
        const onSelect = vi.fn()
        renderTable({ onSelect })
        await user.click(screen.getByText('节点B'))
        expect(onSelect).toHaveBeenCalledWith(data[1])
    })
})