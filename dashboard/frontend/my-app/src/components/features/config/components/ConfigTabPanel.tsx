import { useEffect, useMemo, useState, type ComponentType, type ReactNode } from 'react'
import { AlertTriangle, Database, Plus, Search, SearchX, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
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

interface ConfigTableComponentProps<T> {
    data: T[]
    onSelect: (item: T) => void
    onRename: (item: T) => void
    onDelete: (name: string) => Promise<void>
    selectedName?: string
    selectedIds: string[]
    onSelectionChange: (ids: string[]) => void
    readOnly?: boolean
}

interface ConfigFormComponentProps<TValues> {
    defaultValues: TValues
    onSubmit: (data: TValues) => Promise<void>
    submitLabel?: string
    onDirtyChange?: (dirty: boolean) => void
}

interface ConfigTabPanelProps<TItem, TValues> {
    data: TItem[]
    selectedItem: TItem | null
    onSelect: (item: TItem) => void
    onRename: (item: TItem) => void
    onDelete: (name: string) => Promise<void>
    selectedIds: string[]
    onSelectionChange: (ids: string[]) => void
    TableComponent: ComponentType<ConfigTableComponentProps<TItem>>
    FormComponent: ComponentType<ConfigFormComponentProps<TValues>>
    getFormValues: (item: TItem) => TValues
    typeLabel: string
    listTitle: string
    listDescription: string
    detailDescription: string
    resourceIcon: ReactNode
    onCreate: () => void
    onBatchDelete: () => void
    formSubmit: (data: TValues) => Promise<void>
    readOnly?: boolean
}

export function ConfigTabPanel<
    TItem extends { name: string; displayName: string },
    TValues,
>({
    data,
    selectedItem,
    onSelect,
    onRename,
    onDelete,
    selectedIds,
    onSelectionChange,
    TableComponent,
    FormComponent,
    getFormValues,
    typeLabel,
    listTitle,
    listDescription,
    detailDescription,
    resourceIcon,
    onCreate,
    onBatchDelete,
    formSubmit,
    readOnly = false,
}: ConfigTabPanelProps<TItem, TValues>) {
    const [query, setQuery] = useState('')
    const [formDirty, setFormDirty] = useState(false)
    const [pendingSelection, setPendingSelection] = useState<TItem | null>(null)
    const [pendingCreate, setPendingCreate] = useState(false)

    const filteredData = useMemo(() => {
        const normalized = query.trim().toLocaleLowerCase()
        if (!normalized) return data
        return data.filter((item) =>
            `${item.displayName} ${item.name}`.toLocaleLowerCase().includes(normalized),
        )
    }, [data, query])

    useEffect(() => {
        setFormDirty(false)
        setPendingSelection(null)
        setPendingCreate(false)
    }, [selectedItem?.name])

    const requestSelection = (item: TItem) => {
        if (item.name === selectedItem?.name) return
        if (formDirty) {
            setPendingSelection(item)
            return
        }
        onSelect(item)
    }

    const requestCreate = () => {
        if (readOnly) return
        if (formDirty) {
            setPendingCreate(true)
            return
        }
        onCreate()
    }

    const confirmDiscard = () => {
        if (pendingSelection) onSelect(pendingSelection)
        if (pendingCreate) onCreate()
        setPendingSelection(null)
        setPendingCreate(false)
        setFormDirty(false)
    }

    return (
        <div className="grid h-full min-h-0 grid-cols-1 gap-4 overflow-y-auto p-4 xl:grid-cols-[minmax(390px,44%)_minmax(0,1fr)] xl:overflow-hidden">
            <section className="flex min-h-[420px] flex-col overflow-hidden rounded-xl border border-[#202B3A] bg-[#080C12] shadow-[0_18px_60px_rgba(0,0,0,0.22)] xl:min-h-0">
                <header className="border-b border-[#1B2634] px-4 pb-4 pt-4">
                    <div className="flex items-start justify-between gap-4">
                        <div className="min-w-0">
                            <div className="flex items-center gap-2">
                                <h2 className="text-sm font-semibold text-[#F2F2F2]">{listTitle}</h2>
                                <span className="rounded-full border border-[#2A3548] bg-[#111722] px-2 py-0.5 text-[10px] tabular-nums text-[#8B8B8B]">
                                    {data.length}
                                </span>
                            </div>
                            <p className="mt-1 text-xs leading-5 text-[#6F6F6F]">{listDescription}</p>
                        </div>
                        <Button
                            type="button"
                            size="sm"
                            onClick={requestCreate}
                            disabled={readOnly}
                            title={readOnly ? '历史回放模式下不可新建资源' : undefined}
                            className="h-8 shrink-0 gap-1.5 bg-[#5B8CFF] px-3 text-xs font-medium text-white hover:bg-[#70A0FF] disabled:bg-[#202837] disabled:text-[#596579]"
                        >
                            <Plus className="h-3.5 w-3.5" />
                            新建{typeLabel}
                        </Button>
                    </div>

                    <div className="relative mt-4">
                        <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[#616161]" />
                        <Input
                            value={query}
                            onChange={(event) => setQuery(event.target.value)}
                            placeholder={`搜索${typeLabel}名称或标识`}
                            aria-label={`搜索${typeLabel}`}
                            className="h-9 border-[#263244] bg-[#0D131C] pl-9 text-sm text-[#EDEDED] placeholder:text-[#5E5E5E] focus-visible:border-[#5B8CFF]/60 focus-visible:ring-2 focus-visible:ring-[#5B8CFF]/15"
                        />
                    </div>
                </header>

                {selectedIds.length > 0 && (
                    <div className="flex items-center justify-between border-b border-[#222] bg-[#111827] px-4 py-2.5">
                        <span className="text-xs text-[#A8C7FA]">已选择 {selectedIds.length} 项</span>
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={onBatchDelete}
                            disabled={readOnly}
                            className="h-7 gap-1.5 px-2 text-xs text-[#FF7777] hover:bg-red-500/10 hover:text-[#FF8A8A]"
                        >
                            <Trash2 className="h-3.5 w-3.5" />
                            删除选中
                        </Button>
                    </div>
                )}

                <div className="min-h-0 flex-1 overflow-auto p-3">
                    {filteredData.length > 0 ? (
                        <TableComponent
                            data={filteredData}
                            onSelect={requestSelection}
                            onRename={onRename}
                            onDelete={onDelete}
                            selectedName={selectedItem?.name}
                            selectedIds={selectedIds}
                            onSelectionChange={onSelectionChange}
                            readOnly={readOnly}
                        />
                    ) : data.length === 0 ? (
                        <div className="flex h-full min-h-64 flex-col items-center justify-center rounded-lg border border-dashed border-[#263244] px-6 text-center">
                            <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-[#263244] bg-[#111]">
                                <Database className="h-4 w-4 text-[#737373]" />
                            </div>
                            <p className="mt-4 text-sm font-medium text-[#D6D6D6]">还没有{typeLabel}</p>
                            <p className="mt-1 max-w-64 text-xs leading-5 text-[#6E6E6E]">
                                创建第一项资源后，即可在这里维护详细参数。
                            </p>
                            <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                onClick={requestCreate}
                                disabled={readOnly}
                                className="mt-4 border-[#303C50] bg-[#111722] text-[#D8D8D8] hover:bg-[#1B2634] hover:text-white"
                            >
                                <Plus className="mr-1.5 h-3.5 w-3.5" />
                                新建{typeLabel}
                            </Button>
                        </div>
                    ) : (
                        <div className="flex h-52 flex-col items-center justify-center text-center">
                            <SearchX className="h-5 w-5 text-[#606060]" />
                            <p className="mt-3 text-sm text-[#B7C2D1]">没有匹配的{typeLabel}</p>
                            <button
                                type="button"
                                onClick={() => setQuery('')}
                                className="mt-1 text-xs text-[#5E9EFF] hover:text-[#80B4FF]"
                            >
                                清除搜索条件
                            </button>
                        </div>
                    )}
                </div>
            </section>

            <section className="flex min-h-[520px] min-w-0 flex-col overflow-hidden rounded-xl border border-[#202B3A] bg-[#080C12] shadow-[0_18px_60px_rgba(0,0,0,0.22)] xl:min-h-0">
                {selectedItem ? (
                    <>
                        <header className="flex items-start justify-between gap-4 border-b border-[#1B2634] px-5 py-4">
                            <div className="flex min-w-0 items-start gap-3">
                                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-[#2A3548] bg-[#111722] text-[#A5A5A5]">
                                    {resourceIcon}
                                </div>
                                <div className="min-w-0">
                                    <div className="flex items-center gap-2">
                                        <h2 className="truncate text-sm font-semibold text-[#F2F2F2]">
                                            {selectedItem.displayName}
                                        </h2>
                                        {formDirty && (
                                            <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-[#F5A524]" title="有未保存修改" />
                                        )}
                                    </div>
                                    <div className="mt-1 flex min-w-0 items-center gap-2 text-xs text-[#707070]">
                                        <code className="truncate font-mono">{selectedItem.name}</code>
                                        <span aria-hidden="true">·</span>
                                        <span className="shrink-0">{detailDescription}</span>
                                    </div>
                                </div>
                            </div>
                            <span
                                className={
                                    readOnly
                                        ? 'shrink-0 rounded-full border border-[#7D8FFF]/20 bg-[#7D8FFF]/10 px-2 py-1 text-[10px] font-medium text-[#AEB9FF]'
                                        : 'shrink-0 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2 py-1 text-[10px] font-medium text-[#66D9A3]'
                                }
                            >
                                {readOnly ? '历史只读' : '已持久化'}
                            </span>
                        </header>

                        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
                            <div className="mx-auto w-full max-w-3xl">
                                {readOnly && (
                                    <div className="mb-4 rounded-xl border border-[#7D8FFF]/15 bg-[#7D8FFF]/[0.055] px-3.5 py-3 text-[10px] leading-5 text-[#98A6D9]">
                                        这是历史切面的只读对照视图。返回最新切面后才能修改资源或保存模板。
                                    </div>
                                )}
                                <fieldset
                                    disabled={readOnly}
                                    aria-disabled={readOnly}
                                    className={readOnly ? 'm-0 min-w-0 border-0 p-0 opacity-70' : 'm-0 min-w-0 border-0 p-0'}
                                >
                                    <FormComponent
                                        key={selectedItem.name}
                                        defaultValues={getFormValues(selectedItem)}
                                        onSubmit={formSubmit}
                                        submitLabel={`保存${typeLabel}`}
                                        onDirtyChange={setFormDirty}
                                    />
                                </fieldset>
                            </div>
                        </div>
                    </>
                ) : (
                    <div className="flex h-full min-h-96 flex-col items-center justify-center px-6 text-center">
                        <div className="flex h-12 w-12 items-center justify-center rounded-xl border border-[#263244] bg-[#111] text-[#596579]">
                            {resourceIcon}
                        </div>
                        <h2 className="mt-4 text-sm font-medium text-[#D8D8D8]">选择一个{typeLabel}开始配置</h2>
                        <p className="mt-1 max-w-xs text-xs leading-5 text-[#6E6E6E]">
                            从左侧列表选择资源，或新建一项资源后填写详细参数。
                        </p>
                    </div>
                )}
            </section>

            <AlertDialog
                open={Boolean(pendingSelection) || pendingCreate}
                onOpenChange={(open) => {
                    if (!open) {
                        setPendingSelection(null)
                        setPendingCreate(false)
                    }
                }}
            >
                <AlertDialogContent className="max-w-md border-[#263244] bg-[#111] text-[#F4F4F5]">
                    <AlertDialogHeader>
                        <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-full border border-amber-500/20 bg-amber-500/10">
                            <AlertTriangle className="h-4 w-4 text-[#F5A524]" />
                        </div>
                        <AlertDialogTitle className="text-base">放弃未保存的修改？</AlertDialogTitle>
                        <AlertDialogDescription className="leading-6 text-[#858585]">
                            当前{typeLabel}仍有未保存参数。{pendingCreate ? '新建资源' : '切换资源'}后，这些修改将丢失。
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel className="border-[#303C50] bg-[#141C28] text-[#D4D4D4] hover:bg-[#222] hover:text-white">
                            继续编辑
                        </AlertDialogCancel>
                        <AlertDialogAction onClick={confirmDiscard} className="bg-[#F4F4F5] text-[#111] hover:bg-white">
                            放弃并继续
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </div>
    )
}
