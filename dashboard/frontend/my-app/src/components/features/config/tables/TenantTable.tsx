import type { ColumnDef } from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import { ConfigTable, TruncatedCell } from '../components/ConfigTable'
import type { Tenant, TenantPriority } from '@/types/config.types'

interface TenantTableProps {
    data: Tenant[]
    onSelect: (tenant: Tenant) => void
    onRename?: (tenant: Tenant) => void
    onDelete: (name: string) => Promise<void>
    selectedName?: string
    selectedIds: string[]
    onSelectionChange: (ids: string[]) => void
    readOnly?: boolean
}

const priorityColor: Record<TenantPriority, string> = {
    P1: 'border-red-500/25 bg-red-500/10 text-[#FF7373]',
    P2: 'border-orange-500/25 bg-orange-500/10 text-[#FFAA63]',
    P3: 'border-amber-500/25 bg-amber-500/10 text-[#EBCB66]',
    P4: 'border-blue-500/25 bg-blue-500/10 text-[#73A9FF]',
    P5: 'border-zinc-500/25 bg-zinc-500/10 text-[#A1A1AA]',
}

const formatNumber = new Intl.NumberFormat('zh-CN')

const columns: ColumnDef<Tenant>[] = [
    {
        accessorKey: 'displayName',
        header: '租户',
        cell: ({ row }) => (
            <div className="min-w-0">
                <TruncatedCell className="font-medium text-[#F0F0F0]">{row.original.displayName}</TruncatedCell>
                <TruncatedCell className="mt-0.5 font-mono text-[11px] text-[#596579]">{row.original.name}</TruncatedCell>
            </div>
        ),
    },
    {
        accessorKey: 'priority',
        header: '优先级',
        cell: ({ row }) => (
            <Badge
                variant="outline"
                className={`h-5 rounded px-1.5 text-[10px] font-semibold ${priorityColor[row.original.priority]}`}
            >
                {row.original.priority}
            </Badge>
        ),
    },
    {
        accessorKey: 'qps',
        header: '基准 QPS',
        cell: ({ row }) => (
            <span className="tabular-nums text-[#B7C2D1]">{formatNumber.format(row.original.qps)}</span>
        ),
    },
]

export function TenantTable(props: TenantTableProps) {
    return <ConfigTable {...props} columns={columns} typeLabel="租户" />
}
