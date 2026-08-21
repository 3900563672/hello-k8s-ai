import { useMemo, useState } from 'react'
import { Activity, AlertTriangle, Braces, ListTree, Search } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { useTraceDetail } from '@/api/queries/traceQueries'
import type { TraceSpan, TraceSummary } from '@/types/trace.types'
import { cn } from '@/lib/utils'

const SERVICE_PALETTE = [
    '#5B8CFF',
    '#34D399',
    '#F0A33B',
    '#C084FC',
    '#F472B6',
    '#22D3EE',
    '#A3E635',
    '#FB7185',
]

function serviceColor(service: string): string {
    let hash = 0
    for (let index = 0; index < service.length; index += 1) {
        hash = (hash * 31 + service.charCodeAt(index)) >>> 0
    }
    return SERVICE_PALETTE[hash % SERVICE_PALETTE.length]
}

function formatDuration(durationMs: number): string {
    if (durationMs >= 1000) return `${(durationMs / 1000).toFixed(2)}s`
    if (durationMs >= 1) return `${durationMs.toFixed(1)}ms`
    return `${Math.round(durationMs * 1000)}µs`
}

function formatTime(value: string): string {
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? value : date.toLocaleTimeString('zh-CN', { hour12: false }) + '.' + String(date.getMilliseconds()).padStart(3, '0')
}

function buildSpanTree(spans: TraceSpan[]): { ordered: TraceSpan[]; depth: Map<string, number> } {
    const byParent = new Map<string, TraceSpan[]>()
    const roots: TraceSpan[] = []
    for (const span of spans) {
        if (span.parentSpanId) {
            const list = byParent.get(span.parentSpanId) ?? []
            list.push(span)
            byParent.set(span.parentSpanId, list)
        } else {
            roots.push(span)
        }
    }
    const ordered: TraceSpan[] = []
    const depth = new Map<string, number>()
    const walk = (span: TraceSpan, level: number) => {
        ordered.push(span)
        depth.set(span.spanId, level)
        const children = (byParent.get(span.spanId) ?? []).sort(
            (left, right) => new Date(left.startTime).getTime() - new Date(right.startTime).getTime(),
        )
        for (const child of children) walk(child, level + 1)
    }
    roots.sort((left, right) => new Date(left.startTime).getTime() - new Date(right.startTime).getTime())
    for (const root of roots) walk(root, 0)
    return { ordered, depth }
}

function criticalPath(spans: TraceSpan[]): Set<string> {
    const byParent = new Map<string, TraceSpan[]>()
    const roots: TraceSpan[] = []
    for (const span of spans) {
        if (span.parentSpanId) {
            const list = byParent.get(span.parentSpanId) ?? []
            list.push(span)
            byParent.set(span.parentSpanId, list)
        } else {
            roots.push(span)
        }
    }
    const path = new Set<string>()
    const walk = (span: TraceSpan) => {
        path.add(span.spanId)
        const children = byParent.get(span.spanId) ?? []
        if (children.length === 0) return
        const longest = children.reduce((best, current) =>
            current.durationMs > best.durationMs ? current : best,
        )
        walk(longest)
    }
    const root = roots.sort((left, right) => right.durationMs - left.durationMs)[0]
    if (root) walk(root)
    return path
}

function TraceList({ traces, selectedId, onSelect }: {
    traces: TraceSummary[]
    selectedId: string | null
    onSelect: (traceId: string) => void
}) {
    const [query, setQuery] = useState('')
    const filtered = useMemo(() => {
        const keyword = query.trim().toLowerCase()
        if (!keyword) return traces
        return traces.filter((trace) =>
            trace.traceId.toLowerCase().includes(keyword) ||
            trace.rootOperation.toLowerCase().includes(keyword) ||
            trace.rootService.toLowerCase().includes(keyword),
        )
    }, [traces, query])

    return (
        <div className="flex h-full min-h-0 flex-col">
            <div className="flex items-center gap-2 border-b border-white/[0.06] px-3 py-2">
                <Search className="h-3.5 w-3.5 text-[#6B788C]" />
                <input
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                    placeholder="搜索 Trace ID / 操作 / 服务"
                    className="min-w-0 flex-1 bg-transparent text-[12px] text-[#C6D0DE] outline-none placeholder:text-[#4C5868]"
                />
                <span className="text-[11px] text-[#4C5868]">{filtered.length}</span>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto">
                {filtered.map((trace) => {
                    const active = trace.traceId === selectedId
                    return (
                        <button
                            key={trace.traceId}
                            type="button"
                            onClick={() => onSelect(trace.traceId)}
                            className={cn(
                                'block w-full border-b border-white/[0.04] px-3 py-2 text-left transition-colors',
                                active ? 'bg-[#5B8CFF]/[0.08]' : 'hover:bg-white/[0.03]',
                            )}
                        >
                            <div className="flex items-center justify-between gap-2">
                                <span className="truncate font-mono text-[12px] text-[#9EB2FF]">
                                    {trace.traceId.slice(0, 12)}
                                </span>
                                <span className="flex items-center gap-1">
                                    {trace.errorSpanCount > 0 && (
                                        <AlertTriangle className="h-3 w-3 text-red-400" />
                                    )}
                                    <span className="font-mono text-[11px] text-[#6B788C]">
                                        {formatDuration(trace.durationMs)}
                                    </span>
                                </span>
                            </div>
                            <div className="mt-0.5 truncate text-[12px] text-[#C6D0DE]">
                                {trace.rootOperation}
                            </div>
                            <div className="mt-0.5 flex items-center gap-2 text-[11px] text-[#5A6778]">
                                <span>{trace.rootService}</span>
                                <span>{trace.spanCount} spans</span>
                                <span>{formatTime(trace.startTime)}</span>
                            </div>
                        </button>
                    )
                })}
                {filtered.length === 0 && (
                    <p className="px-3 py-6 text-center text-[12px] text-[#4C5868]">无匹配 Trace</p>
                )}
            </div>
        </div>
    )
}

function SpanDetailPanel({ span, serviceColorOf }: {
    span: TraceSpan
    serviceColorOf: (service: string) => string
}) {
    return (
        <div className="flex h-full min-h-0 flex-col overflow-y-auto border-l border-white/[0.06]">
            <div className="border-b border-white/[0.06] px-3 py-2.5">
                <div className="flex items-center gap-2">
                    <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: serviceColorOf(span.service) }} />
                    <span className="truncate text-[12px] font-medium text-[#D5DFEC]">{span.operation}</span>
                </div>
                <div className="mt-1 truncate text-[11px] text-[#5A6778]">{span.service}</div>
            </div>
            <div className="space-y-4 px-3 py-3">
                <div className="grid grid-cols-2 gap-2">
                    <div className="rounded-lg bg-white/[0.025] px-2.5 py-2">
                        <div className="text-[11px] text-[#5A6778]">耗时</div>
                        <div className="mt-0.5 font-mono text-[12px] text-[#C6D0DE]">{formatDuration(span.durationMs)}</div>
                    </div>
                    <div className="rounded-lg bg-white/[0.025] px-2.5 py-2">
                        <div className="text-[11px] text-[#5A6778]">状态</div>
                        <div className={cn(
                            'mt-0.5 text-[12px]',
                            span.status === 'error' || span.status === 'fail'
                                ? 'text-red-300'
                                : 'text-emerald-300',
                        )}>
                            {span.status}
                        </div>
                    </div>
                </div>

                <div>
                    <div className="flex items-center gap-1.5 text-[11px] font-medium text-[#8C99AC]">
                        <Braces className="h-3 w-3" />
                        属性
                    </div>
                    <div className="mt-1.5 space-y-1">
                        {Object.entries(span.attributes).map(([key, value]) => (
                            <div key={key} className="flex items-start justify-between gap-2 rounded-lg bg-white/[0.02] px-2 py-1.5">
                                <span className="shrink-0 text-[11px] text-[#6B788C]">{key}</span>
                                <span className="min-w-0 break-all text-right font-mono text-[11px] text-[#AEB9C8]">
                                    {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                                </span>
                            </div>
                        ))}
                        {Object.keys(span.attributes).length === 0 && (
                            <p className="text-[11px] text-[#4C5868]">无属性</p>
                        )}
                    </div>
                </div>

                <div>
                    <div className="flex items-center gap-1.5 text-[11px] font-medium text-[#8C99AC]">
                        <Activity className="h-3 w-3" />
                        事件
                    </div>
                    <div className="mt-1.5 space-y-1">
                        {span.events.map((event, index) => (
                            <div key={index} className="rounded-lg bg-white/[0.02] px-2 py-1.5">
                                <div className="flex items-center justify-between">
                                    <span className="text-[11px] text-[#AEB9C8]">{event.name}</span>
                                    <span className="font-mono text-[10px] text-[#5A6778]">{formatTime(event.time)}</span>
                                </div>
                                {event.attributes && Object.keys(event.attributes).length > 0 && (
                                    <div className="mt-1 break-all font-mono text-[10px] text-[#6B788C]">
                                        {JSON.stringify(event.attributes)}
                                    </div>
                                )}
                            </div>
                        ))}
                        {span.events.length === 0 && (
                            <p className="text-[11px] text-[#4C5868]">无事件</p>
                        )}
                    </div>
                </div>
            </div>
        </div>
    )
}

export function TraceWaterfall({ traces }: { traces: TraceSummary[] }) {
    const [selectedTraceId, setSelectedTraceId] = useState<string | null>(null)
    const [selectedSpanId, setSelectedSpanId] = useState<string | null>(null)

    const effectiveTraceId = selectedTraceId ?? traces[0]?.traceId ?? null
    const detailQuery = useTraceDetail(effectiveTraceId)
    const spans = useMemo(() => detailQuery.data?.data.spans ?? [], [detailQuery.data])

    const { ordered, depth } = useMemo(() => buildSpanTree(spans), [spans])
    const critical = useMemo(() => criticalPath(spans), [spans])

    const timeRange = useMemo(() => {
        if (ordered.length === 0) return null
        const starts = ordered.map((span) => new Date(span.startTime).getTime())
        return {
            min: Math.min(...starts),
            max: Math.max(...starts.map((start, index) => start + ordered[index].durationMs)),
        }
    }, [ordered])

    const selectedSpan = ordered.find((span) => span.spanId === selectedSpanId) ?? null

    return (
        <div className="flex h-[520px] min-h-0 flex-col gap-3">
            <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden rounded-xl border border-white/[0.07] bg-[#090D14]/90 md:grid-cols-[300px_minmax(0,1fr)]">
                <div className="hidden min-h-0 overflow-hidden border-r border-white/[0.06] md:block">
                    <TraceList
                        traces={traces}
                        selectedId={effectiveTraceId}
                        onSelect={(traceId) => {
                            setSelectedTraceId(traceId)
                            setSelectedSpanId(null)
                        }}
                    />
                </div>

                <div className="flex min-h-0 flex-col">
                    {effectiveTraceId && ordered.length > 0 && timeRange ? (
                        <>
                            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-white/[0.06] px-3 py-2">
                                <span className="font-mono text-[12px] text-[#9EB2FF]">{effectiveTraceId.slice(0, 12)}</span>
                                <span className="text-[11px] text-[#6B788C]">{ordered.length} spans</span>
                                <span className="text-[11px] text-[#6B788C]">{formatDuration(timeRange.max - timeRange.min)}</span>
                                <span className="text-[11px] text-[#6B788C]">{formatTime(ordered[0].startTime)}</span>
                                {ordered.some((span) => span.status === 'error' || span.status === 'fail') && (
                                    <Badge variant="outline" className="h-5 border-red-400/20 bg-red-400/10 px-1.5 text-[11px] text-red-300">
                                        含错误 Span
                                    </Badge>
                                )}
                                <div className="flex-1" />
                                <span className="hidden items-center gap-1.5 text-[11px] text-[#4C5868] lg:flex">
                                    <span className="h-1.5 w-3 rounded-sm bg-white/15" />
                                    关键路径
                                </span>
                            </div>

                            <div className="relative h-10 shrink-0 border-b border-white/[0.06] px-3 pt-2">
                                <div className="relative h-5 overflow-hidden rounded-sm bg-white/[0.03]">
                                    {ordered.map((span) => {
                                        const start = new Date(span.startTime).getTime()
                                        const left = ((start - timeRange.min) / (timeRange.max - timeRange.min)) * 100
                                        const width = Math.max(0.4, (span.durationMs / (timeRange.max - timeRange.min)) * 100)
                                        return (
                                            <span
                                                key={span.spanId}
                                                className="absolute top-0 h-full rounded-[1px]"
                                                style={{
                                                    left: `${left}%`,
                                                    width: `${width}%`,
                                                    backgroundColor: span.status === 'error' || span.status === 'fail'
                                                        ? 'rgba(248,113,113,0.75)'
                                                        : serviceColor(span.service),
                                                    opacity: 0.8,
                                                }}
                                                title={`${span.operation} · ${formatDuration(span.durationMs)}`}
                                            />
                                        )
                                    })}
                                </div>
                            </div>

                            <div className="min-h-0 flex-1 overflow-y-auto">
                                {ordered.map((span) => {
                                    const start = new Date(span.startTime).getTime()
                                    const left = ((start - timeRange.min) / (timeRange.max - timeRange.min)) * 100
                                    const width = Math.max(0.5, (span.durationMs / (timeRange.max - timeRange.min)) * 100)
                                    const indent = (depth.get(span.spanId) ?? 0) * 18
                                    const isCritical = critical.has(span.spanId)
                                    const isSelected = span.spanId === selectedSpanId
                                    const color = serviceColor(span.service)
                                    const hasError = span.status === 'error' || span.status === 'fail'
                                    return (
                                        <button
                                            key={span.spanId}
                                            type="button"
                                            onClick={() => setSelectedSpanId(span.spanId)}
                                            className={cn(
                                                'relative block h-7 w-full text-left transition-colors',
                                                isSelected ? 'bg-[#5B8CFF]/[0.1]' : 'hover:bg-white/[0.03]',
                                            )}
                                        >
                                            <span
                                                className="absolute top-1/2 h-[2px] -translate-y-1/2 rounded-full"
                                                style={{
                                                    left: `${left}%`,
                                                    width: `${width}%`,
                                                    backgroundColor: hasError ? '#F87171' : color,
                                                    boxShadow: isCritical ? `0 0 6px ${color}` : 'none',
                                                    outline: isCritical ? `1px solid ${color}` : 'none',
                                                    opacity: hasError ? 0.95 : 0.8,
                                                }}
                                            />
                                            <span
                                                className="absolute left-0 top-1/2 flex -translate-y-1/2 items-center gap-1.5 pr-3"
                                                style={{ paddingLeft: `${left + 10}px`, marginLeft: indent }}
                                            >
                                                <span className="h-1.5 w-1.5 shrink-0 rounded-full" style={{ backgroundColor: color }} />
                                                <span className="truncate text-[11px] text-[#B9C4D3]">{span.operation}</span>
                                                <span className="shrink-0 font-mono text-[10px] text-[#5A6778]">
                                                    {formatDuration(span.durationMs)}
                                                </span>
                                                {isCritical && (
                                                    <span className="shrink-0 text-[9px] uppercase tracking-wider text-[#5A6778]">
                                                        critical
                                                    </span>
                                                )}
                                            </span>
                                        </button>
                                    )
                                })}
                            </div>
                        </>
                    ) : (
                        <div className="flex flex-1 items-center justify-center">
                            <div className="text-center">
                                <ListTree className="mx-auto h-6 w-6 text-[#3E4A5C]" />
                                <p className="mt-2 text-[12px] text-[#5A6778]">
                                    {traces.length === 0 ? '暂无 Trace 数据' : '选择左侧 Trace 查看瀑布图'}
                                </p>
                            </div>
                        </div>
                    )}
                </div>
            </div>

            {selectedSpan && (
                <div className="h-[240px] shrink-0 overflow-hidden rounded-xl border border-white/[0.07] bg-[#090D14]/90">
                    <SpanDetailPanel span={selectedSpan} serviceColorOf={serviceColor} />
                </div>
            )}

            <p className="text-[12px] text-[#4C5868]">
                瀑布图为结构化调用链视图：行内缩进表示父子关系，条宽表示耗时，高亮条为关键路径。移动端可点击列表页签切换 Trace。
            </p>
        </div>
    )
}
