import type { ColumnDef } from '@tanstack/react-table'
import { ConfigTable, TruncatedCell } from '../components/ConfigTable'
import type { Node } from '@/types/config.types'

interface NodeTableProps {
    data: Node[]
    onSelect: (node: Node) => void
    onRename: (node: Node) => void
    onDelete: (name: string) => Promise<void>
    selectedName?: string
    selectedIds: string[]
    onSelectionChange: (ids: string[]) => void
    readOnly?: boolean
}

const formatNumber = new Intl.NumberFormat('zh-CN')

const columns: ColumnDef<Node>[] = [
    {
        accessorKey: 'displayName',
        header: '节点',
        cell: ({ row }) => (
            <div className="min-w-0">
                <TruncatedCell className="font-medium text-[#F0F0F0]">{row.original.displayName}</TruncatedCell>
                <TruncatedCell className="mt-0.5 font-mono text-[11px] text-[#596579]">{row.original.name}</TruncatedCell>
            </div>
        ),
    },
    {
        accessorKey: 'gpu',
        header: '总显存',
        cell: ({ row }) => (
            <span className="tabular-nums text-[#B7C2D1]">{formatNumber.format(row.original.gpu)} G</span>
        ),
    },
    {
        accessorKey: 'maxConcurrency',
        header: '并发',
        cell: ({ row }) => (
            <span className="tabular-nums text-[#B7C2D1]">{formatNumber.format(row.original.maxConcurrency)}</span>
        ),
    },
]

export function NodeTable(props: NodeTableProps) {
    return <ConfigTable {...props} columns={columns} typeLabel="节点" />
}
