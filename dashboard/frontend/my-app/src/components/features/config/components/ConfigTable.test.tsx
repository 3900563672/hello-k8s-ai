import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ConfigTable, NameCell } from '@/components/features/config/components/ConfigTable'
import type { ColumnDef } from '@tanstack/react-table'
import { TooltipProvider } from '@/components/ui/tooltip'

interface Row {
    name: string
    displayName: string
    gpu: number
}

const columns: ColumnDef<Row>[] = [
    {
        accessorKey: 'displayName',
        header: '显示名称',
        cell: ({ row }) => <NameCell displayName={row.original.displayName} name={row.original.name} />,
    },
    {
        accessorKey: 'gpu',
        header: 'GPU',
        cell: ({ row }) => <span>{row.original.gpu}</span>,
    },
]

const rows: Row[] = [
    { name: 'node-1', displayName: '生产节点 A', gpu: 8 },
    { name: 'node-2', displayName: '生产节点 B', gpu: 4 },
]

const renderTable = (props: Partial<Parameters<typeof ConfigTable<Row>>[0]> = {}) =>
    render(
        <TooltipProvider>
            <ConfigTable<Row>
                data={rows}
                columns={columns}
                selectedIds={[]}
                onSelectionChange={vi.fn()}
                onSelect={vi.fn()}
                onDelete={vi.fn(async () => undefined)}
                onRename={vi.fn()}
                typeLabel="节点"
                {...props}
            />
        </TooltipProvider>,
    )

describe('ConfigTable', () => {
    it('渲染行数据与显示名称', () => {
        renderTable()
        expect(screen.getByText('生产节点 A')).toBeInTheDocument()
        expect(screen.getByText('node-1')).toBeInTheDocument()
        expect(screen.getByText('生产节点 B')).toBeInTheDocument()
        expect(screen.getByText('8')).toBeInTheDocument()
    })

    it('点击行触发 onSelect', async () => {
        const user = userEvent.setup()
        const onSelect = vi.fn()
        renderTable({ onSelect })
        await user.click(screen.getByText('生产节点 A'))
        expect(onSelect).toHaveBeenCalledWith(rows[0])
    })

    it('勾选行与全选更新选择集', async () => {
        const user = userEvent.setup()
        const onSelectionChange = vi.fn()
        renderTable({ onSelectionChange })
        await user.click(screen.getByLabelText('选择 生产节点 A'))
        expect(onSelectionChange).toHaveBeenCalledWith(['node-1'])
        await user.click(screen.getByLabelText('选择当前列表'))
        expect(onSelectionChange).toHaveBeenCalledWith(['node-1', 'node-2'])
    })

    it('readOnly 时复选框禁用', () => {
        renderTable({ readOnly: true })
        expect(screen.getByLabelText('选择当前列表')).toBeDisabled()
    })

    it('删除流程：操作菜单 → 确认弹窗 → 调用 onDelete', async () => {
        const user = userEvent.setup()
        const onDelete = vi.fn(async () => undefined)
        renderTable({ onDelete })
        await user.click(screen.getByRole('button', { name: '打开 生产节点 A 的操作菜单' }))
        await user.click(await screen.findByRole('menuitem', { name: /删除/ }))
        expect(await screen.findByText('删除节点“生产节点 A”？')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: '确认删除' }))
        expect(onDelete).toHaveBeenCalledWith('node-1')
    })
})

describe('NameCell', () => {
    it('渲染显示名称与资源名', () => {
        render(
            <TooltipProvider>
                <NameCell displayName="租户 A" name="tenant-a" />
            </TooltipProvider>,
        )
        expect(screen.getByText('租户 A')).toBeInTheDocument()
        expect(screen.getByText('tenant-a')).toBeInTheDocument()
    })
})