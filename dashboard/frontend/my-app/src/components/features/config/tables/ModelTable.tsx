import type { ColumnDef } from '@tanstack/react-table'
import { ConfigTable, NameCell, formatNumber } from '../components/ConfigTable'
import type { Model } from '@/types/config.types'

interface ModelTableProps {
    data: Model[]
    onSelect: (model: Model) => void
    onRename?: (model: Model) => void
    onDelete: (name: string) => Promise<void>
    selectedName?: string
    selectedIds: string[]
    onSelectionChange: (ids: string[]) => void
    readOnly?: boolean
}

const columns: ColumnDef<Model>[] = [
    {
        accessorKey: 'displayName',
        header: '模型',
        cell: ({ row }) => (
            <NameCell displayName={row.original.displayName} name={row.original.name} />
        ),
    },
    {
        accessorKey: 'gpuUnits',
        header: '显存',
        cell: ({ row }) => (
            <span className="tabular-nums text-[#B7C2D1]">{formatNumber.format(row.original.gpuUnits)} G</span>
        ),
    },
    {
        accessorKey: 'maxConcurrency',
        header: '并发',
        cell: ({ row }) => (
            <span className="tabular-nums text-[#B7C2D1]">{formatNumber.format(row.original.maxConcurrency)}</span>
        ),
    },
    {
        accessorKey: 'absoluteScore',
        header: '基准分',
        cell: ({ row }) => row.original.absoluteScore > 0 ? (
            <span className="tabular-nums text-[#B7C2D1]">{formatNumber.format(row.original.absoluteScore)}</span>
        ) : (
            <span className="text-[#FFB86B]">待配置</span>
        ),
    },
]

export function ModelTable(props: ModelTableProps) {
    return <ConfigTable {...props} columns={columns} typeLabel="模型" />
}
