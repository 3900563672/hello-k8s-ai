import type { ColumnDef } from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import { ConfigTable, NameCell, formatNumber } from '../components/ConfigTable'
import type { Orchestrator } from '@/types/config.types'

interface OrchestratorTableProps {
    data: Orchestrator[]
    onSelect: (orchestrator: Orchestrator) => void
    onRename?: (orchestrator: Orchestrator) => void
    onDelete: (name: string) => Promise<void>
    selectedName?: string
    selectedIds: string[]
    onSelectionChange: (ids: string[]) => void
    readOnly?: boolean
}

const columns: ColumnDef<Orchestrator>[] = [
    {
        accessorKey: 'displayName',
        header: '关联租户',
        cell: ({ row }) => (
            <NameCell displayName={row.original.displayName} name={row.original.name} />
        ),
    },
    {
        accessorKey: 'minReplicas',
        header: '副本范围',
        cell: ({ row }) => (
            <span className="tabular-nums text-[#B7C2D1]">
                {formatNumber.format(row.original.minReplicas)} – {row.original.maxReplicas === 0 ? '∞' : formatNumber.format(row.original.maxReplicas)}
            </span>
        ),
    },
    {
        accessorKey: 'scaleUpCooldownSeconds',
        header: '扩容冷却',
        cell: ({ row }) => (
            <span className="tabular-nums text-[#B7C2D1]">{row.original.scaleUpCooldownSeconds}s</span>
        ),
    },
    {
        accessorKey: 'scaleDownCooldownSeconds',
        header: '缩容冷却',
        cell: ({ row }) => (
            <span className="tabular-nums text-[#B7C2D1]">{row.original.scaleDownCooldownSeconds}s</span>
        ),
    },
    {
        accessorKey: 'allowScaleToZero',
        header: '缩到零',
        cell: ({ row }) => (
            <Badge
                variant="outline"
                className={`h-5 rounded px-1.5 text-[10px] font-semibold ${
                    row.original.allowScaleToZero
                        ? 'border-emerald-500/25 bg-emerald-500/10 text-[#57C894]'
                        : 'border-zinc-500/25 bg-zinc-500/10 text-[#A1A1AA]'
                }`}
            >
                {row.original.allowScaleToZero ? '允许' : '禁止'}
            </Badge>
        ),
    },
]

export function OrchestratorTable(props: OrchestratorTableProps) {
    return <ConfigTable {...props} columns={columns} typeLabel="编排策略" />
}
