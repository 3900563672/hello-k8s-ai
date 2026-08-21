import { useState, type KeyboardEvent, type ReactNode } from 'react'
import {
    flexRender,
    getCoreRowModel,
    useReactTable,
    type CellContext,
    type ColumnDef,
} from '@tanstack/react-table'
import { AlertTriangle, Loader2, LockKeyhole, MoreHorizontal, Pencil, Trash2 } from 'lucide-react'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

interface TruncatedCellProps {
    children: ReactNode
    className?: string
}

export function TruncatedCell({ children, className = '' }: TruncatedCellProps) {
    const text = String(children ?? '')
    if (!text) return <span className="text-[#5F5F5F]">—</span>

    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <span className={`block truncate ${className}`}>{text}</span>
            </TooltipTrigger>
            <TooltipContent side="top" className="max-w-sm border-[#2A3548] bg-[#141C28] text-[#F4F4F5]">
                {text}
            </TooltipContent>
        </Tooltip>
    )
}

export const formatNumber = new Intl.NumberFormat('zh-CN')

// NameCell 渲染「显示名称 + 资源名」的两行单元格，各配置表共用。
export function NameCell({ displayName, name }: { displayName: string; name: string }) {
    return (
        <div className="min-w-0">
            <TruncatedCell className="font-medium text-[#F0F0F0]">{displayName}</TruncatedCell>
            <TruncatedCell className="mt-0.5 font-mono text-[14px] text-[#596579]">{name}</TruncatedCell>
        </div>
    )
}

interface ConfigTableProps<T extends { name: string; displayName: string }> {
    data: T[]
    columns: ColumnDef<T>[]
    selectedIds: string[]
    onSelectionChange: (ids: string[]) => void
    onSelect: (item: T) => void
    onRename?: (item: T) => void
    onDelete: (name: string) => Promise<void>
    selectedName?: string
    typeLabel: string
    skipTruncate?: string[]
    readOnly?: boolean
}

const errorMessage = (error: unknown): string =>
    error instanceof Error ? error.message : '操作失败，请稍后重试'

export function ConfigTable<T extends { name: string; displayName: string }>({
    data,
    columns: customColumns,
    selectedIds,
    onSelectionChange,
    onSelect,
    onRename,
    onDelete,
    selectedName,
    typeLabel,
    skipTruncate = [],
    readOnly = false,
}: ConfigTableProps<T>) {
    const [deleteTarget, setDeleteTarget] = useState<T | null>(null)
    const [deletePending, setDeletePending] = useState(false)
    const [deleteError, setDeleteError] = useState('')

    const visibleIds = data.map((item) => item.name)
    const visibleIdSet = new Set(visibleIds)
    const selectedVisibleCount = selectedIds.filter((id) => visibleIdSet.has(id)).length
    const allVisibleSelected = data.length > 0 && selectedVisibleCount === data.length
    const someVisibleSelected = selectedVisibleCount > 0 && !allVisibleSelected

    const toggleAll = () => {
        if (readOnly) return
        if (allVisibleSelected) {
            onSelectionChange(selectedIds.filter((id) => !visibleIdSet.has(id)))
            return
        }
        onSelectionChange(Array.from(new Set([...selectedIds, ...visibleIds])))
    }

    const toggleItem = (name: string) => {
        if (readOnly) return
        onSelectionChange(
            selectedIds.includes(name)
                ? selectedIds.filter((id) => id !== name)
                : [...selectedIds, name],
        )
    }

    const enhancedColumns: ColumnDef<T>[] = [
        {
            id: 'select',
            header: () => (
                <Checkbox
                    checked={someVisibleSelected ? 'indeterminate' : allVisibleSelected}
                    onCheckedChange={toggleAll}
                    disabled={readOnly}
                    aria-label={allVisibleSelected ? '取消选择当前列表' : '选择当前列表'}
                    className="border-[#484848] data-[state=checked]:border-[#5B8CFF] data-[state=checked]:bg-[#5B8CFF] data-[state=indeterminate]:border-[#5B8CFF] data-[state=indeterminate]:bg-[#5B8CFF]"
                />
            ),
            cell: ({ row }) => (
                <Checkbox
                    checked={selectedIds.includes(row.original.name)}
                    onCheckedChange={() => toggleItem(row.original.name)}
                    disabled={readOnly}
                    onClick={(event) => event.stopPropagation()}
                    aria-label={`选择 ${row.original.displayName}`}
                    className="border-[#484848] data-[state=checked]:border-[#5B8CFF] data-[state=checked]:bg-[#5B8CFF]"
                />
            ),
        },
        ...customColumns.map((column) => {
            const accessorKey = (column as ColumnDef<T> & { accessorKey?: string }).accessorKey
            if (column.cell || !accessorKey || skipTruncate.includes(accessorKey)) return column

            return {
                ...column,
                cell: (context: CellContext<T, unknown>) => (
                    <TruncatedCell>{context.getValue() as ReactNode}</TruncatedCell>
                ),
            } as ColumnDef<T>
        }),
        {
            id: 'actions',
            cell: ({ row }) => {
                const item = row.original
                if (readOnly) {
                    return (
                        <span className="flex h-8 w-8 items-center justify-center text-[#4E5A6C]" title="历史回放只读">
                            <LockKeyhole className="h-3.5 w-3.5" />
                        </span>
                    )
                }
                return (
                    <DropdownMenu modal={false}>
                        <DropdownMenuTrigger asChild>
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                aria-label={`打开 ${item.displayName} 的操作菜单`}
                                onClick={(event) => event.stopPropagation()}
                                className="h-8 w-8 text-[#748196] opacity-0 transition group-hover:opacity-100 hover:bg-[#202B3A] hover:text-white focus-visible:opacity-100 data-[state=open]:bg-[#202B3A] data-[state=open]:text-white data-[state=open]:opacity-100"
                            >
                                <MoreHorizontal className="h-4 w-4" />
                            </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-40 border-[#263244] bg-[#101722] p-1 text-[#EDEDED]">
                            {onRename ? (
                                <DropdownMenuItem
                                    onSelect={() => onRename(item)}
                                    className="cursor-pointer gap-2 focus:bg-[#252525] focus:text-white"
                                >
                                    <Pencil className="h-3.5 w-3.5 text-[#8A8A8A]" />
                                    重命名
                                </DropdownMenuItem>
                            ) : null}
                            <DropdownMenuItem
                                onSelect={() => {
                                    setDeleteError('')
                                    setDeleteTarget(item)
                                }}
                                className="cursor-pointer gap-2 text-[#FF6B6B] focus:bg-red-500/10 focus:text-[#FF8080]"
                            >
                                <Trash2 className="h-3.5 w-3.5" />
                                删除
                            </DropdownMenuItem>
                        </DropdownMenuContent>
                    </DropdownMenu>
                )
            },
        },
    ]

    const table = useReactTable({
        data,
        columns: enhancedColumns,
        getCoreRowModel: getCoreRowModel(),
        getRowId: (row) => row.name,
    })

    const getWidth = (columnId: string): string | number | undefined => {
        if (columnId === 'select') return 42
        if (columnId === 'actions') return 46
        if (columnId === 'displayName') return '46%'
        return undefined
    }

    const confirmDelete = async () => {
        if (!deleteTarget || deletePending) return
        setDeletePending(true)
        setDeleteError('')
        try {
            await onDelete(deleteTarget.name)
            setDeleteTarget(null)
        } catch (error) {
            setDeleteError(errorMessage(error))
        } finally {
            setDeletePending(false)
        }
    }

    return (
        <TooltipProvider delayDuration={350}>
            <div className="overflow-hidden rounded-lg border border-[#202B3A] bg-[#0B1018]">
                <Table className="w-full table-fixed">
                    <TableHeader className="bg-[#0D131C]">
                        {table.getHeaderGroups().map((headerGroup) => (
                            <TableRow key={headerGroup.id} className="border-b border-[#202B3A] hover:bg-transparent">
                                {headerGroup.headers.map((header) => (
                                    <TableHead
                                        key={header.id}
                                        className="h-9 px-3 text-[14px] font-medium uppercase tracking-[0.08em] text-[#747474]"
                                        style={{ width: getWidth(header.column.id) }}
                                    >
                                        {header.isPlaceholder
                                            ? null
                                            : flexRender(header.column.columnDef.header, header.getContext())}
                                    </TableHead>
                                ))}
                            </TableRow>
                        ))}
                    </TableHeader>
                    <TableBody>
                        {table.getRowModel().rows.map((row) => {
                            const active = row.original.name === selectedName
                            return (
                                <TableRow
                                    key={row.id}
                                    tabIndex={0}
                                    aria-selected={active}
                                    onClick={() => onSelect(row.original)}
                                    onKeyDown={(event: KeyboardEvent<HTMLTableRowElement>) => {
                                        if (event.key === 'Enter' || event.key === ' ') {
                                            event.preventDefault()
                                            onSelect(row.original)
                                        }
                                    }}
                                    className={`group relative cursor-pointer border-b border-[#1E1E1E] outline-none transition-colors last:border-0 focus-visible:bg-[#141C28] ${
                                        active
                                            ? 'bg-[#10223A] hover:bg-[#132944] [&>td:first-child]:shadow-[inset_2px_0_0_#5B8CFF]'
                                            : 'hover:bg-[#101722]'
                                    }`}
                                >
                                    {row.getVisibleCells().map((cell) => (
                                        <TableCell
                                            key={cell.id}
                                            className="h-[58px] overflow-hidden px-3 py-2 text-sm text-[#D8D8D8]"
                                            style={{ width: getWidth(cell.column.id) }}
                                        >
                                            {flexRender(cell.column.columnDef.cell, cell.getContext())}
                                        </TableCell>
                                    ))}
                                </TableRow>
                            )
                        })}
                    </TableBody>
                </Table>
            </div>

            <AlertDialog
                open={Boolean(deleteTarget)}
                onOpenChange={(open) => {
                    if (!open && !deletePending) {
                        setDeleteTarget(null)
                        setDeleteError('')
                    }
                }}
            >
                <AlertDialogContent className="max-w-md border-[#263244] bg-[#0D131C] p-0 text-[#FAFAFA] shadow-2xl">
                    <AlertDialogHeader className="px-6 pb-2 pt-6 text-left">
                        <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-full border border-red-500/20 bg-red-500/10">
                            <AlertTriangle className="h-5 w-5 text-[#FF6B6B]" />
                        </div>
                        <AlertDialogTitle className="text-base font-semibold">
                            删除{typeLabel}“{deleteTarget?.displayName}”？
                        </AlertDialogTitle>
                        <AlertDialogDescription className="leading-6 text-[#8A8A8A]">
                            此操作会从本地配置中永久移除该{typeLabel}，且无法撤销。
                        </AlertDialogDescription>
                        {deleteError && (
                            <div className="mt-3 rounded-md border border-red-500/20 bg-red-500/10 px-3 py-2 text-sm text-[#FF8A8A]">
                                {deleteError}
                            </div>
                        )}
                    </AlertDialogHeader>
                    <AlertDialogFooter className="border-t border-[#202B3A] bg-[#080C12] px-6 py-4 sm:justify-end">
                        <AlertDialogCancel
                            disabled={deletePending}
                            className="border-[#303C50] bg-[#141C28] text-[#D4D4D4] hover:bg-[#222] hover:text-white"
                        >
                            取消
                        </AlertDialogCancel>
                        <AlertDialogAction
                            onClick={(event) => {
                                event.preventDefault()
                                void confirmDelete()
                            }}
                            disabled={deletePending}
                            className="min-w-24 bg-[#E5484D] font-medium text-white hover:bg-[#F2555A]"
                        >
                            {deletePending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                            确认删除
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </TooltipProvider>
    )
}
