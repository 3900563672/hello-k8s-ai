import type { ColumnDef } from '@tanstack/react-table'
import { ConfigTable, NameCell, formatNumber } from '../components/ConfigTable'
import type { Node } from '@/types/config.types'

interface NodeTableProps {
    data: Node[]
    onSelect: (node: Node) => void
    onRename?: (node: Node) => void
    onDelete: (name: string) => Promise<void>
    selectedName?: string
    selectedIds: string[]
    onSelectionChange: (ids: string[]) => void
    readOnly?: boolean
}

const columns: ColumnDef<Node>[] = [
    {
        accessorKey: 'displayName',
        header: '节点',
        cell: ({ row }) => (
            <NameCell displayName={row.original.displayName} name={row.original.name} />
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
