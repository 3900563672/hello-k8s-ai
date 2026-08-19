import { useEffect, useMemo, useRef, useState } from 'react'
import {
    DndContext,
    DragOverlay,
    type DragEndEvent,
    type DragStartEvent,
} from '@dnd-kit/core'
import {
    Activity,
    AlertCircle,
    CheckCircle2,
    Clock3,
    GitCompareArrows,
    Layers3,
    UserRound,
    X,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useSetTenantTraffic, useTenants } from '@/api/queries/trafficQueries'
import { useTrafficStore } from '@/stores/trafficSlice'
import { cn } from '@/lib/utils'
import type { TrafficTemplate, TrafficViewMode } from '@/types/traffic.types'
import { ApplyOverlayDialog } from './ApplyOverlayDialog'
import { CanvasDropZone } from './CanvasDropZone'
import { DrawCanvas } from './DrawCanvas'
import { OverlayList } from './OverlayList'
import { TemplateLibrary } from './TemplateLibrary'
import {
    formatLogicalTime,
    getScenarioHorizon,
    getTemplateValueAtTime,
} from './trafficMath'
import { useReplayTimeContext } from '@/stores/timeSlice'
import { formatUtcTimestamp } from '@/lib/formatters/timeFormatter'

export function TrafficPage() {
    const replay = useReplayTimeContext()
    const {
        viewMode,
        setViewMode,
        selectedTenant,
        setSelectedTenant,
        compareTenants,
        toggleCompareTenant,
        templates,
        overlays,
        addTemplate,
        addOverlay,
        mode,
        setMode,
    } = useTrafficStore()
    const { data: tenants = [] } = useTenants()
    const applyTraffic = useSetTenantTraffic()
    const [draggedTemplate, setDraggedTemplate] = useState<TrafficTemplate | null>(null)
    const [applyTemplate, setApplyTemplate] = useState<TrafficTemplate | null>(null)
    const [notice, setNotice] = useState<{ message: string; kind: 'success' | 'error' } | null>(null)
    const noticeTimer = useRef<number | null>(null)

    useEffect(() => () => {
        if (noticeTimer.current !== null) window.clearTimeout(noticeTimer.current)
    }, [])

    const activeOverlays = useMemo(
        () => overlays.filter((overlay) =>
            overlay.enabled && templates.some((template) => template.id === overlay.templateId),
        ),
        [overlays, templates],
    )
    const activeTenantCount = useMemo(
        () => new Set(activeOverlays.map((overlay) => overlay.tenantId)).size,
        [activeOverlays],
    )
    const horizon = useMemo(
        () => getScenarioHorizon(templates, activeOverlays),
        [activeOverlays, templates],
    )

    const showNotice = (message: string, kind: 'success' | 'error' = 'success') => {
        setNotice({ message, kind })
        if (noticeTimer.current !== null) window.clearTimeout(noticeTimer.current)
        noticeTimer.current = window.setTimeout(() => setNotice(null), 2800)
    }

    const openApply = (template: TrafficTemplate) => setApplyTemplate(template)

    const handleDragStart = (event: DragStartEvent) => {
        setDraggedTemplate(templates.find((template) => template.id === event.active.id) ?? null)
    }

    const handleDragEnd = (event: DragEndEvent) => {
        if (event.over?.id === 'traffic-canvas-dropzone') {
            const template = templates.find((item) => item.id === event.active.id)
            if (template) openApply(template)
        }
        setDraggedTemplate(null)
    }

    const handleApply = (data: {
        templateId: string
        tenantId: string
        startOffsetSeconds: number
    }) => {
        const template = templates.find((item) => item.id === data.templateId)
        const tenant = tenants.find((item) => item.id === data.tenantId)
        if (!template || !tenant) return
        if (replay.mode === 'historical') {
            showNotice('历史模式只读，不能应用流量', 'error')
            return
        }
        const increment = getTemplateValueAtTime(template, 0)
        const targetQps = Math.max(0, Math.round((tenant.requestedQPS ?? 0) + increment))
        applyTraffic.mutate(
            { tenantId: tenant.id, qps: targetQps },
            {
                onSuccess: () => {
                    addOverlay({
                        templateId: template.id,
                        templateName: template.name,
                        tenantId: tenant.id,
                        tenantName: tenant.name,
                        startOffsetSeconds: data.startOffsetSeconds,
                        effectiveAt: replay.effectiveAt,
                        snapshotId: replay.snapshotId,
                        enabled: true,
                    })
                    setApplyTemplate(null)
                    showNotice(
                        `已将“${template.name}”叠加到 ${tenant.name}，目标 QPS 已写入（${targetQps}）`,
                    )
                },
                onError: (error) => {
                    const message = error instanceof Error ? error.message : '应用流量失败'
                    showNotice(`应用失败：${message}`, 'error')
                },
            },
        )
    }

    if (mode === 'draw') {
        return (
            <DrawCanvas
                onSave={(name, points) => {
                    addTemplate({
                        name,
                        shapeType: 'custom',
                        controlPoints: points,
                        description: '使用逻辑时间与真实 QPS 坐标绘制的自定义模板',
                    })
                    setMode('overview')
                    showNotice(`模板“${name}”已保存`)
                }}
                onCancel={() => setMode('overview')}
            />
        )
    }

    const copy = getViewCopy(viewMode, selectedTenant, tenants)

    return (
        <DndContext onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
            <div className="flex h-full min-h-[640px] flex-col overflow-hidden bg-[#06090E] text-[#E8EEF7]">
                <header className="shrink-0 border-b border-white/[0.07] bg-[#080C12]/95">
                    <div className="flex min-h-[58px] items-center justify-between gap-4 px-5">
                        <div className="flex min-w-0 items-center gap-4">
                            <div className="flex items-center gap-2.5">
                                <div className="flex h-9 w-9 items-center justify-center rounded-xl border border-[#5B8CFF]/25 bg-[#5B8CFF]/10 text-[#7DB3FF] shadow-[0_0_26px_rgba(91,140,255,.10)]">
                                    <Activity className="h-[17px] w-[17px]" />
                                </div>
                                <div className="hidden xl:block">
                                    <div className="text-xs font-semibold text-[#E8EEF7]">流量布置</div>
                                    <div className="mt-0.5 text-[9px] uppercase tracking-[0.15em] text-[#536073]">Traffic Composer</div>
                                </div>
                            </div>

                            <div className="h-6 w-px bg-white/[0.07]" />

                            <Tabs
                                value={viewMode}
                                onValueChange={(value: string) => setViewMode(value as TrafficViewMode)}
                            >
                                <TabsList className="h-9 rounded-xl border border-white/[0.06] bg-white/[0.025] p-1">
                                    <ViewTab value="total" icon={<Layers3 className="h-3.5 w-3.5" />} label="总 QPS" />
                                    <ViewTab value="single" icon={<UserRound className="h-3.5 w-3.5" />} label="单租户" />
                                    <ViewTab value="compare" icon={<GitCompareArrows className="h-3.5 w-3.5" />} label="租户对比" />
                                </TabsList>
                            </Tabs>

                            {viewMode === 'single' && (
                                <Select
                                    value={selectedTenant ?? ''}
                                    onValueChange={(value: string) => setSelectedTenant(value || null)}
                                >
                                    <SelectTrigger className="h-9 w-[180px] border-white/[0.08] bg-white/[0.025] text-xs text-[#DCE5F0]">
                                        <SelectValue placeholder="选择租户（不预选）" />
                                    </SelectTrigger>
                                    <SelectContent className="border-white/[0.08] bg-[#0E131C] text-[#E6EDF6]">
                                        {tenants.map((tenant) => (
                                            <SelectItem key={tenant.id} value={tenant.id}>{tenant.name}</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            )}

                            {viewMode === 'compare' && (
                                <div className="flex min-w-0 items-center gap-1.5 overflow-x-auto">
                                    {tenants.map((tenant) => {
                                        const selected = compareTenants.includes(tenant.id)
                                        return (
                                            <Badge
                                                key={tenant.id}
                                                variant="outline"
                                                onClick={() => toggleCompareTenant(tenant.id)}
                                                className={cn(
                                                    'h-7 shrink-0 cursor-pointer gap-1 border-white/[0.08] bg-white/[0.02] px-2 text-[10px] font-normal text-[#788599] transition-colors hover:bg-white/[0.05] hover:text-white',
                                                    selected && 'border-[#5B8CFF]/35 bg-[#5B8CFF]/12 text-[#9BC5FF]',
                                                )}
                                            >
                                                {tenant.name}
                                                {selected && <X className="h-2.5 w-2.5" />}
                                            </Badge>
                                        )
                                    })}
                                </div>
                            )}
                        </div>

                        <div
                            className="flex shrink-0 items-center gap-2 rounded-lg border border-white/[0.06] bg-white/[0.02] px-2.5 py-1.5 text-[9px] text-[#687487]"
                            title={`${replay.effectiveAt} · ${replay.snapshotId ?? '无切面 ID'}`}
                        >
                            <Clock3 className="h-3 w-3 text-[#7292BF]" />
                            <span>{replay.mode === 'historical' ? '历史基线' : '最新基线'}</span>
                            <span className="hidden font-mono font-medium text-[#9DB9DE] 2xl:inline">
                                {formatUtcTimestamp(replay.effectiveAt).slice(5)} UTC
                            </span>
                            <span className="font-mono font-medium text-[#9DB9DE] 2xl:hidden">
                                {replay.mode === 'historical' ? 'HIST' : 'LIVE'}
                            </span>
                        </div>
                    </div>

                    <div className="flex min-h-[44px] items-center justify-between gap-4 border-t border-white/[0.045] px-5">
                        <div className="min-w-0">
                            <div className="truncate text-xs font-medium text-[#DCE5F0]">{copy.title}</div>
                            <div className="mt-0.5 truncate text-[10px] text-[#5E6A7D]">{copy.description}</div>
                        </div>
                        <div className="flex shrink-0 items-center gap-5 text-[9px] text-[#5E6A7D]">
                            <HeaderMetric label="启用叠加" value={`${activeOverlays.length}`} />
                            <HeaderMetric label="有流量租户" value={`${activeTenantCount}`} />
                            <HeaderMetric label="当前视域" value={`T+${formatLogicalTime(horizon)}`} />
                        </div>
                    </div>
                </header>

                <div className="flex min-h-0 flex-1">
                    <aside className="w-[272px] shrink-0 border-r border-white/[0.07] bg-[#080C12]">
                        <TemplateLibrary onApply={openApply} />
                    </aside>

                    <main className="flex min-w-0 flex-1 flex-col bg-[#06090E]">
                        <div className="min-h-0 flex-1 p-3 pb-2">
                            <CanvasDropZone />
                        </div>
                        <div className="h-[102px] shrink-0 border-t border-white/[0.07]">
                            <OverlayList />
                        </div>
                    </main>
                </div>
            </div>

            <DragOverlay dropAnimation={{ duration: 180, easing: 'ease-out' }}>
                {draggedTemplate && (
                    <div className="flex items-center gap-2 rounded-xl border border-[#5B8CFF]/35 bg-[#0D1522]/95 px-3.5 py-2.5 text-xs text-[#DDEBFF] shadow-2xl backdrop-blur-xl">
                        <span className="h-2 w-2 rounded-full bg-[#6DA6FF] shadow-[0_0_12px_rgba(91,140,255,.8)]" />
                        {draggedTemplate.name}
                    </div>
                )}
            </DragOverlay>

            <ApplyOverlayDialog
                open={applyTemplate !== null}
                onOpenChange={(open: boolean) => !open && setApplyTemplate(null)}
                template={applyTemplate}
                tenants={tenants}
                pending={applyTraffic.isPending}
                onApply={handleApply}
            />

            {notice && (
                <div className={`fixed bottom-5 right-5 z-[100] flex items-center gap-2.5 rounded-xl border px-4 py-3 text-xs shadow-2xl backdrop-blur-xl ${
                    notice.kind === 'error'
                        ? 'border-red-400/25 bg-[#1A0D10]/95 text-red-100'
                        : 'border-emerald-400/20 bg-[#0A1713]/95 text-emerald-100'
                }`}>
                    {notice.kind === 'error'
                        ? <AlertCircle className="h-4 w-4 text-red-400" />
                        : <CheckCircle2 className="h-4 w-4 text-emerald-400" />}
                    {notice.message}
                </div>
            )}
        </DndContext>
    )
}

function ViewTab({ value, icon, label }: { value: TrafficViewMode; icon: React.ReactNode; label: string }) {
    return (
        <TabsTrigger
            value={value}
            className="h-7 gap-1.5 rounded-lg px-3 text-[10px] text-[#778397] data-[state=active]:bg-[#5B8CFF]/15 data-[state=active]:text-[#A9CDFF] data-[state=active]:shadow-none"
        >
            {icon}
            {label}
        </TabsTrigger>
    )
}

function HeaderMetric({ label, value }: { label: string; value: string }) {
    return (
        <div className="flex items-center gap-1.5">
            <span>{label}</span>
            <span className="font-mono font-medium text-[#A0AEC0]">{value}</span>
        </div>
    )
}

function getViewCopy(
    mode: TrafficViewMode,
    selectedTenant: string | null,
    tenants: Array<{ id: string; name: string }>,
) {
    if (mode === 'total') {
        return {
            title: '系统总 QPS',
            description: '严格等于所有租户最终 QPS 的逐点之和',
        }
    }
    if (mode === 'single') {
        const tenant = tenants.find((item) => item.id === selectedTenant)
        return {
            title: tenant?.name ?? '单租户流量',
            description: tenant ? '基础值为 0，所有已启用模板按逻辑偏移做纯加法' : '请选择租户后查看独立流量',
        }
    }
    return {
        title: '租户流量对比',
        description: '各租户独立计算，不堆叠、不共享模板实例',
    }
}
