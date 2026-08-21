import type { ColumnDef } from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import { ConfigTable, NameCell } from '../components/ConfigTable'
import type { Policy, PolicyKind } from '@/types/config.types'

interface PolicyTableProps {
    data: Policy[]
    onSelect: (policy: Policy) => void
    onDelete: (name: string) => Promise<void>
    selectedName?: string
    selectedIds: string[]
    onSelectionChange: (ids: string[]) => void
    readOnly?: boolean
}

const kindLabels: Record<PolicyKind, string> = {
    tenantModel: '租户-模型',
    tenantNode: '租户-节点',
    modelNode: '模型-节点',
}

const columns: ColumnDef<Policy>[] = [
    {
        accessorKey: 'displayName',
        header: '引用关系',
        cell: ({ row }) => (
            <NameCell displayName={row.original.displayName} name={row.original.name} />
        ),
    },
    {
        accessorKey: 'kind',
        header: '类型',
        cell: ({ row }) => (
            <Badge
                variant="outline"
                className="h-5 rounded border-[#263244] bg-[#121826] px-1.5 text-[12px] font-semibold text-[#8CB8F8]"
            >
                {kindLabels[row.original.kind]}
            </Badge>
        ),
    },
    {
        accessorKey: 'effect',
        header: '效果',
        cell: ({ row }) => (
            <Badge
                variant="outline"
                className={`h-5 rounded px-1.5 text-[12px] font-semibold ${
                    row.original.effect === 'Allow'
                        ? 'border-emerald-500/25 bg-emerald-500/10 text-[#57C894]'
                        : 'border-red-500/25 bg-red-500/10 text-[#FF7373]'
                }`}
            >
                {row.original.effect === 'Allow' ? '允许' : '禁止'}
            </Badge>
        ),
    },
]

export function PolicyTable(props: PolicyTableProps) {
    return <ConfigTable {...props} columns={columns} typeLabel="策略" />
}