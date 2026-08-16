import type { ColumnDef } from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import { ConfigTable, TruncatedCell } from '../components/ConfigTable'
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

const formatNumber = new Intl.NumberFormat('zh-CN')

const columns: ColumnDef<Orchestrator>[] = [
    {
        accessorKey: 'displayName',
        header: '关联租户',
        cell: ({ row }) => (
            <div className="min-w-0">
                <TruncatedCell className="font-medium text-[#F0F0F0]">{row.original.displayName}</TruncatedCell>
                <TruncatedCell className="mt-0.5 font-mono text-[11px] text-[#596579]">{row.original.name}</TruncatedCell>
            </div>
        ),
    },
    {
        accessorKey: 'minReplicas',
        header: '副本范围',
        cell: ({ row }) => (
            <span className="tabular-nums text-[#B7C2D1]">
                {formatNumber.format(row.original.minReplicas)} – {formatNumber.format(row.original.maxReplicas)}
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
