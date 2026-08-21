import type { ColumnDef } from '@tanstack/react-table'
import { ConfigTable, NameCell, formatNumber } from '../components/ConfigTable'
import type { Node } from '@/types/config.types'

function GpuUsageBar({ used, total }: { used: number | undefined; total: number }) {
    if (used === undefined) {
        return <span className="text-[#5F5F5F]">—</span>
    }
    const ratio = total > 0 ? used / total : 0
    const tone = ratio > 0.9 ? 'bg-[#EF5B67]' : ratio > 0.7 ? 'bg-[#F6B73C]' : 'bg-[#34C77B]'
    return (
        <div className="min-w-0">
            <div className="flex items-baseline gap-1.5">
                <span className="font-mono tabular-nums text-[#B7C2D1]">{formatNumber.format(used)}</span>
                <span className="text-[12px] text-[#596579]">/ {formatNumber.format(total)} G</span>
            </div>
            <div className="mt-1 h-1 overflow-hidden rounded-full bg-white/[0.06]">
                <div className={`h-full rounded-full ${tone}`} style={{ width: `${Math.min(100, ratio * 100)}%` }} />
            </div>
        </div>
    )
}

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
        header: 'GPU 用量',
        cell: ({ row }) => (
            <GpuUsageBar used={(row.original.status as { usedGPU?: number } | undefined)?.usedGPU} total={row.original.gpu} />
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
