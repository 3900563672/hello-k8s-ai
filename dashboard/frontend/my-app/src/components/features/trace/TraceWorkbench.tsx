import { useMemo, useRef, useState } from 'react'
import {
    AlertTriangle,
    CheckCircle2,
    Copy,
    Crosshair,
    Database,
    Filter,
    GripVertical,
    History,
    Layers,
    RefreshCw,
    Search,
} from 'lucide-react'
import ReactECharts from 'echarts-for-react'
import { useSegment } from '@/api/queries/traceQueries'
import { useTimeStore } from '@/stores/timeSlice'
import type { Snapshot, SnapshotDomain, SnapshotSeverity } from '@/types/time.types'
import type { MetricResult, SegmentOverviewData } from '@/types/trace.types'
import { aggregateMetricPoints, metricStats } from '@/lib/formatters/metrics'

const dateTime = new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
})

const number = new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 })

function formatTime(value?: string) {
    if (!value) return '—'
    const parsed = new Date(value)
    return Number.isNaN(parsed.getTime()) ? value : dateTime.format(parsed)
}

const severityMeta: Record<SnapshotSeverity, { label: string; dot: string; pill: string }> = {
    normal: { label: '正常', dot: 'bg-emerald-400', pill: 'border-emerald-400/20 bg-emerald-400/[0.07] text-emerald-300' },
    attention: { label: '关注', dot: 'bg-amber-300', pill: 'border-amber-300/25 bg-amber-300/[0.08] text-amber-200' },
    critical: { label: '严重', dot: 'bg-red-400', pill: 'border-red-400/25 bg-red-400/[0.08] text-red-300' },
}

const domainLabels: Record<SnapshotDomain, string> = {
    scheduler: '调度器',
    configuration: '配置',
    capacity: '容量',
    runtime: '运行时',
}

function typeLabel(type: Snapshot['type']) {
    return type === 'config' ? '配置决策' : '事件'
}

function MetricMiniChart({ metric }: { metric: MetricResult }) {
    const points = useMemo(() => aggregateMetricPoints(metric), [metric])
    const option = {
        animation: false,
        grid: { left: 2, right: 2, top: 4, bottom: 2 },
        xAxis: { type: 'time', show: false },
        yAxis: { type: 'value', show: false, scale: true },
        series: [{
            type: 'line',
            data: points.map((point) => [new Date(point.time).getTime(), point.value]),
            showSymbol: false,
            lineStyle: { color: '#5B8CFF', width: 1.2 },
            areaStyle: {
                color: {
                    type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
                    colorStops: [
                        { offset: 0, color: 'rgba(91,140,255,0.18)' },
                        { offset: 1, color: 'rgba(91,140,255,0)' },
                    ],
                },
            },
        }],
    }
    return <ReactECharts option={option} style={{ height: 64 }} notMerge />
}

function snapshotSummary(snapshot: SegmentOverviewData['startSnapshot']) {
    if (!snapshot) return null
    const pods = snapshot.workloads.pods
    const deployments = snapshot.workloads.deployments
    const readyPods = pods.filter((pod) => pod.ready).length
    const readyDeployments = deployments.filter(
        (deployment) => deployment.readyReplicas >= deployment.desiredReplicas,
    ).length
    return {
        pods: pods.length,
        readyPods,
        deployments: deployments.length,
        readyDeployments,
        nodes: snapshot.workloads.nodes.length,
        tenants: snapshot.traffic.tenants.length,
        models: snapshot.configuration.models.length,
    }
}

function StatCell({ label, value, tone }: { label: string; value: string | number; tone?: 'ok' | 'warn' }) {
    return (
        <div className="min-w-0">
            <div className="truncate text-[12px] uppercase tracking-[0.1em] text-[#52607a]">{label}</div>
            <div className={`mt-0.5 font-mono text-[14px] tabular-nums ${tone === 'ok' ? 'text-emerald-300' : tone === 'warn' ? 'text-amber-200' : 'text-[#D5DFEC]'}`}>
                {value}
            </div>
        </div>
    )
}

function FilterSection({ title, options, value, onChange }: {
    title: string
    options: Array<{ value: string; label: string; dot?: string }>
    value: string[]
    onChange: (next: string[]) => void
}) {
    const toggle = (item: string) => {
        onChange(value.includes(item) ? value.filter((v) => v !== item) : [...value, item])
    }
    return (
        <div className="border-b border-white/[0.05] px-3 py-3">
            <div className="mb-2 flex items-center justify-between">
                <span className="text-[13px] font-medium uppercase tracking-[0.12em] text-[#5b6b82]">{title}</span>
                {value.length > 0 && (
                    <button type="button" onClick={() => onChange([])} className="text-[12px] text-[#4f5d71] hover:text-[#93a6bd]">清除</button>
                )}
            </div>
            <div className="space-y-1">
                {options.map((option) => {
                    const active = value.includes(option.value)
                    return (
                        <button
                            key={option.value}
                            type="button"
                            onClick={() => toggle(option.value)}
                            className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[12px] transition-colors ${
                                active ? 'bg-[#5B8CFF]/[0.09] text-[#CFE0F7]' : 'text-[#7d8ba0] hover:bg-white/[0.03] hover:text-[#b6c3d4]'
                            }`}
                        >
                            {option.dot && <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${option.dot}`} />}
                            <span className="flex-1">{option.label}</span>
                            {active && <span className="h-1 w-1 rounded-full bg-[#5B8CFF]" />}
                        </button>
                    )
                })}
            </div>
        </div>
    )
}

const rangeOptions = [
    { value: 'all', label: '全部' },
    { value: '1h', label: '1 小时' },
    { value: '30m', label: '30 分钟' },
]
export function TraceWorkbench() {
    const snapshots = useTimeStore((state) => state.snapshots)
    const selectedId = useTimeStore((state) => state.selectedSnapshotId)
    const selectSnapshot = useTimeStore((state) => state.selectSnapshot)

    const [query, setQuery] = useState('')
    const [severities, setSeverities] = useState<string[]>([])
    const [types, setTypes] = useState<string[]>([])
    const [domains, setDomains] = useState<string[]>([])
    const [range, setRange] = useState('all')
    const [drawerPct, setDrawerPct] = useState(40)
    const [tab, setTab] = useState<'snapshot' | 'decisions' | 'metrics' | 'anomalies'>('snapshot')
    const [copied, setCopied] = useState(false)

    const containerRef = useRef<HTMLDivElement>(null)
    const dragRef = useRef<{ startX: number; startPct: number } | null>(null)

    const ordered = useMemo(
        () => [...snapshots].sort(
            (left, right) => new Date(right.timestamp).getTime() - new Date(left.timestamp).getTime(),
        ),
        [snapshots],
    )

    const filtered = useMemo(() => {
        const latest = snapshots.at(-1)?.timestamp
        const cutoff = range === 'all' || !latest
            ? null
            : new Date(new Date(latest).getTime() - (range === '1h' ? 3_600_000 : 1_800_000)).getTime()
        const keyword = query.trim().toLowerCase()
        return ordered.filter((snapshot) => {
            if (keyword && !`${snapshot.title} ${snapshot.summary} ${snapshot.source}`.toLowerCase().includes(keyword)) return false
            if (severities.length > 0 && !severities.includes(snapshot.severity)) return false
            if (types.length > 0 && !types.includes(snapshot.type)) return false
            if (domains.length > 0 && !domains.includes(snapshot.domain)) return false
            if (cutoff !== null && new Date(snapshot.timestamp).getTime() < cutoff) return false
            return true
        })
    }, [ordered, snapshots, query, severities, types, domains, range])

    const selected = useMemo(
        () => snapshots.find((snapshot) => snapshot.id === selectedId) ?? null,
        [snapshots, selectedId],
    )

    const windowStart = useMemo(() => {
        if (!selected) return null
        return new Date(new Date(selected.timestamp).getTime() - 30 * 60_000).toISOString()
    }, [selected])

    const segmentResult = useSegment(
        windowStart && selected ? { start: windowStart, end: selected.timestamp } : null,
    )
    const segment = segmentResult.data?.data

    const windowItems = useMemo(() => {
        if (!selected) return []
        const end = new Date(selected.timestamp).getTime()
        const start = end - 30 * 60_000
        return snapshots
            .filter((snapshot) => {
                const time = new Date(snapshot.timestamp).getTime()
                return time >= start && time <= end
            })
            .sort((left, right) => new Date(left.timestamp).getTime() - new Date(right.timestamp).getTime())
    }, [snapshots, selected])

    const decisions = useMemo(
        () => windowItems.filter((snapshot) => snapshot.type === 'config'),
        [windowItems],
    )
    const anomalies = useMemo(
        () => windowItems.filter((snapshot) => snapshot.severity !== 'normal'),
        [windowItems],
    )

    const startSummary = snapshotSummary(segment?.startSnapshot)
    const endSummary = snapshotSummary(segment?.endSnapshot)

    const onPointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
        dragRef.current = { startX: event.clientX, startPct: drawerPct }
        event.currentTarget.setPointerCapture(event.pointerId)
    }
    const onPointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
        if (!dragRef.current || !containerRef.current) return
        const rect = containerRef.current.getBoundingClientRect()
        if (rect.width === 0) return
        const deltaPct = ((event.clientX - dragRef.current.startX) / rect.width) * 100
        setDrawerPct(Math.min(55, Math.max(28, dragRef.current.startPct - deltaPct)))
    }
    const onPointerUp = () => {
        dragRef.current = null
    }

    const copyId = async () => {
        if (!selected) return
        try {
            await navigator.clipboard.writeText(selected.id)
            setCopied(true)
            window.setTimeout(() => setCopied(false), 1200)
        } catch {
            // 剪贴板不可用时静默
        }
    }

    const countBySeverity = (severity: SnapshotSeverity) =>
        filtered.filter((snapshot) => snapshot.severity === severity).length

    return (
        <div ref={containerRef} className="flex min-h-[480px] gap-3">
            {/* 左：筛选 */}
            <aside className="hidden w-[212px] shrink-0 self-start overflow-hidden rounded-xl border border-white/[0.06] bg-panel/80 lg:block">
                <div className="flex items-center gap-2 border-b border-white/[0.05] px-3 py-2.5">
                    <Filter className="h-3 w-3 text-[#5B8CFF]" />
                    <span className="text-[12px] font-medium text-[#AEBBD0]">筛选</span>
                    <span className="ml-auto font-mono text-[12px] text-[#52607a]">{filtered.length} 条</span>
                </div>
                <div className="border-b border-white/[0.05] p-3">
                    <div className="relative">
                        <Search className="absolute left-2 top-1/2 h-3 w-3 -translate-y-1/2 text-[#4f5d71]" />
                        <input
                            value={query}
                            onChange={(event) => setQuery(event.target.value)}
                            placeholder="搜索标题 / 摘要…"
                            className="h-8 w-full rounded-lg border border-white/[0.07] bg-panel-2 pl-7 pr-2 text-[12px] text-[#C6D3E4] outline-none placeholder:text-[#4a5a72] focus:border-[#5B8CFF]/40"
                        />
                    </div>
                </div>
                <FilterSection
                    title="严重度"
                    value={severities}
                    onChange={setSeverities}
                    options={[
                        { value: 'normal', label: '正常', dot: 'bg-emerald-400' },
                        { value: 'attention', label: '关注', dot: 'bg-amber-300' },
                        { value: 'critical', label: '严重', dot: 'bg-red-400' },
                    ]}
                />
                <FilterSection
                    title="类型"
                    value={types}
                    onChange={setTypes}
                    options={[
                        { value: 'config', label: '配置决策' },
                        { value: 'event', label: '事件' },
                    ]}
                />
                <FilterSection
                    title="域"
                    value={domains}
                    onChange={setDomains}
                    options={[
                        { value: 'scheduler', label: '调度器' },
                        { value: 'runtime', label: '运行时' },
                        { value: 'configuration', label: '配置' },
                        { value: 'capacity', label: '容量' },
                    ]}
                />
                <div className="px-3 py-3">
                    <div className="mb-2 text-[13px] font-medium uppercase tracking-[0.12em] text-[#5b6b82]">时间范围</div>
                    <div className="grid grid-cols-3 gap-1">
                        {rangeOptions.map((option) => (
                            <button
                                key={option.value}
                                type="button"
                                onClick={() => setRange(option.value)}
                                className={`rounded-md px-1 py-1.5 text-[13px] transition-colors ${
                                    range === option.value
                                        ? 'bg-[#5B8CFF]/[0.12] text-[#C8DBF5]'
                                        : 'bg-white/[0.025] text-[#6b7a90] hover:bg-white/[0.05]'
                                }`}
                            >
                                {option.label}
                            </button>
                        ))}
                    </div>
                </div>
            </aside>

            {/* 中：切面列表 */}
            <main className="flex min-w-0 flex-1 flex-col overflow-hidden rounded-xl border border-white/[0.06] bg-panel/80">
                <div className="flex items-center gap-3 border-b border-white/[0.05] px-3 py-2">
                    <History className="h-3.5 w-3.5 text-[#5B8CFF]" />
                    <span className="text-[14px] font-semibold text-[#D3DFEE]">切面时间线</span>
                    <span className="rounded-full border border-white/[0.07] bg-white/[0.03] px-2 py-0.5 font-mono text-[12px] text-[#7d8ba0]">
                        {filtered.length} / {ordered.length}
                    </span>
                    <div className="ml-auto flex items-center gap-2">
                        <span className="hidden items-center gap-2 text-[12px] text-[#52607a] sm:flex">
                            <span className="flex items-center gap-1"><span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />{countBySeverity('normal')}</span>
                            <span className="flex items-center gap-1"><span className="h-1.5 w-1.5 rounded-full bg-amber-300" />{countBySeverity('attention')}</span>
                            <span className="flex items-center gap-1"><span className="h-1.5 w-1.5 rounded-full bg-red-400" />{countBySeverity('critical')}</span>
                        </span>
                    </div>
                </div>
                {filtered.length === 0 ? (
                    <div className="flex flex-1 flex-col items-center justify-center gap-2 px-6 py-12 text-center">
                        <Layers className="h-5 w-5 text-[#3d4c63]" />
                        <div className="text-[12px] text-[#5b6b82]">没有匹配的切面</div>
                        <div className="text-[13px] text-[#47566d]">调整筛选条件或时间范围后重试</div>
                    </div>
                ) : (
                    <div className="max-h-[720px] min-h-0 flex-1 overflow-y-auto">
                        {filtered.map((snapshot) => {
                            const meta = severityMeta[snapshot.severity] ?? severityMeta.normal
                            const active = snapshot.id === selectedId
                            return (
                                <button
                                    key={snapshot.id}
                                    type="button"
                                    onClick={() => selectSnapshot(snapshot.id)}
                                    className={`flex w-full items-center gap-2.5 border-l-2 px-3 py-2.5 text-left transition-colors ${
                                        active
                                            ? 'border-[#5B8CFF] bg-[#5B8CFF]/[0.08]'
                                            : 'border-transparent hover:bg-white/[0.025]'
                                    }`}
                                >
                                    <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${meta.dot} ${active ? 'shadow-[0_0_6px_rgba(91,140,255,.6)]' : ''}`} />
                                    <span className="min-w-0 flex-1">
                                        <span className={`block truncate text-[12px] ${active ? 'text-[#E4EDF8]' : 'text-[#B9C6D8]'}`}>
                                            {snapshot.title}
                                        </span>
                                        <span className="mt-0.5 block truncate font-mono text-[12px] text-[#54637c]">
                                            {formatTime(snapshot.timestamp)}
                                        </span>
                                    </span>
                                    <span className="shrink-0 rounded border border-white/[0.06] bg-white/[0.02] px-1.5 py-0.5 text-[12px] text-[#64748d]">
                                        {typeLabel(snapshot.type)}
                                    </span>
                                    <span className="hidden shrink-0 text-[12px] text-[#55657e] md:block">
                                        {domainLabels[snapshot.domain] ?? snapshot.domain}
                                    </span>
                                </button>
                            )
                        })}
                    </div>
                )}
            </main>
            {/* 拖拽分隔条 */}
            <div
                role="separator"
                aria-orientation="vertical"
                onPointerDown={onPointerDown}
                onPointerMove={onPointerMove}
                onPointerUp={onPointerUp}
                className="group hidden w-1.5 shrink-0 cursor-col-resize items-center justify-center self-stretch rounded-full transition-colors hover:bg-[#5B8CFF]/20 lg:flex"
                title="拖拽调整详情宽度"
            >
                <GripVertical className="h-3 w-3 text-[#3d4c63] group-hover:text-[#7ca5f0]" />
            </div>

            {/* 右：详情抽屉 */}
            <aside
                className="hidden shrink-0 flex-col overflow-hidden rounded-xl border border-white/[0.06] bg-panel/90 lg:flex"
                style={{ width: `calc(${drawerPct}% - 6px)` }}
            >
                {!selected ? (
                    <div className="flex flex-1 flex-col items-center justify-center gap-2 px-6 text-center">
                        <Crosshair className="h-5 w-5 text-[#3d4c63]" />
                        <div className="text-[12px] text-[#5b6b82]">选择一个切面查看详情</div>
                        <div className="text-[13px] text-[#47566d]">摘要、前后快照对照、决策序列与演化曲线</div>
                    </div>
                ) : (
                    <>
                        <div className="border-b border-white/[0.05] p-3.5">
                            <div className="flex flex-wrap items-center gap-1.5">
                                <span className={`rounded-full border px-2 py-0.5 text-[12px] font-medium ${severityMeta[selected.severity]?.pill ?? severityMeta.normal.pill}`}>
                                    {severityMeta[selected.severity]?.label ?? '正常'}
                                </span>
                                <span className="rounded-full border border-[#5B8CFF]/20 bg-[#5B8CFF]/[0.06] px-2 py-0.5 text-[12px] text-[#AFCBFF]">
                                    {typeLabel(selected.type)}
                                </span>
                                <span className="rounded-full border border-white/[0.07] bg-white/[0.02] px-2 py-0.5 text-[12px] text-[#7d8ba0]">
                                    {domainLabels[selected.domain] ?? selected.domain}
                                </span>
                                <span className="ml-auto font-mono text-[12px] text-[#52607a]">{formatTime(selected.timestamp)}</span>
                            </div>
                            <h3 className="mt-2.5 text-[14px] font-semibold leading-5 tracking-[-0.01em] text-[#EEF3FA]">{selected.title}</h3>
                            <p className="mt-1.5 text-[12px] leading-[18px] text-[#8b9ab1]">{selected.summary}</p>
                            <div className="mt-2 flex items-center gap-1.5 font-mono text-[12px] text-[#4f5f78]">
                                <Database className="h-3 w-3" />
                                <span className="truncate">{selected.source}</span>
                            </div>
                            <div className="mt-2.5 grid grid-cols-4 gap-2 rounded-lg border border-white/[0.05] bg-panel-2/60 p-2.5">
                                <StatCell label="租户" value={selected.impact.tenants} />
                                <StatCell label="节点" value={selected.impact.nodes} />
                                <StatCell label="模型" value={selected.impact.models} />
                                <StatCell label="变更" value={selected.impact.changes} tone={selected.impact.changes > 0 ? 'warn' : 'ok'} />
                            </div>
                            {selected.tags.length > 0 && (
                                <div className="mt-2.5 flex flex-wrap gap-1">
                                    {selected.tags.map((tag) => (
                                        <span key={tag} className="rounded border border-white/[0.06] bg-white/[0.02] px-1.5 py-0.5 font-mono text-[12px] text-[#64748d]">
                                            {tag}
                                        </span>
                                    ))}
                                </div>
                            )}
                        </div>

                        <div className="flex border-b border-white/[0.05] px-2 pt-1.5">
                            {([
                                ['snapshot', '快照对照', segment ? '1' : ''],
                                ['decisions', '决策序列', String(decisions.length)],
                                ['metrics', '演化曲线', ''],
                                ['anomalies', '异常', String(anomalies.length)],
                            ] as const).map(([key, label, count]) => (
                                <button
                                    key={key}
                                    type="button"
                                    onClick={() => setTab(key)}
                                    className={`-mb-px border-b-2 px-3 py-2 text-[13px] transition-colors ${
                                        tab === key
                                            ? 'border-[#5B8CFF] text-[#D3E0F0]'
                                            : 'border-transparent text-[#5f6f87] hover:text-[#9fb0c7]'
                                    }`}
                                >
                                    {label}
                                    {count !== '' && <span className="ml-1 font-mono text-[12px] text-[#4f5f78]">{count}</span>}
                                </button>
                            ))}
                        </div>

                        <div className="min-h-0 flex-1 overflow-y-auto p-3.5">
                            {tab === 'snapshot' && (
                                segmentResult.isPending ? (
                                    <div className="flex items-center justify-center gap-2 py-10 text-[13px] text-[#5b6b82]">
                                        <RefreshCw className="h-3.5 w-3.5 animate-spin text-[#5B8CFF]" />
                                        正在聚合时间段快照…
                                    </div>
                                ) : segmentResult.isError ? (
                                    <div className="rounded-lg border border-red-400/15 bg-red-400/[0.04] p-3 text-[13px] text-red-200/80">
                                        时间段分析不可用：{segmentResult.error.message}
                                    </div>
                                ) : !segment ? (
                                    <div className="py-8 text-center text-[13px] text-[#5b6b82]">时间段数据不可用</div>
                                ) : (
                                    <div className="space-y-3">
                                        <div className="rounded-lg border border-white/[0.05] bg-panel-2/60 px-3 py-2.5">
                                            <div className="flex items-center justify-between text-[12px] text-[#52607a]">
                                                <span>时间段切面</span>
                                                <span className="font-mono">{formatTime(segment.start)} → {formatTime(segment.end)}</span>
                                            </div>
                                            {startSummary && endSummary ? (
                                                <div className="mt-3 grid grid-cols-2 gap-3">
                                                    <div>
                                                        <div className="mb-2 flex items-center gap-1.5 text-[12px] font-medium uppercase tracking-[0.1em] text-[#6b7a90]">
                                                            <span className="h-1.5 w-1.5 rounded-full bg-[#5B8CFF]" /> 起点
                                                        </div>
                                                        <div className="grid grid-cols-2 gap-x-2 gap-y-2.5">
                                                            <StatCell label="Pod" value={`${startSummary.readyPods}/${startSummary.pods}`} tone={startSummary.readyPods === startSummary.pods ? 'ok' : 'warn'} />
                                                            <StatCell label="Deploy" value={`${startSummary.readyDeployments}/${startSummary.deployments}`} tone={startSummary.readyDeployments === startSummary.deployments ? 'ok' : 'warn'} />
                                                            <StatCell label="节点" value={startSummary.nodes} />
                                                            <StatCell label="租户" value={startSummary.tenants} />
                                                        </div>
                                                    </div>
                                                    <div>
                                                        <div className="mb-2 flex items-center gap-1.5 text-[12px] font-medium uppercase tracking-[0.1em] text-[#6b7a90]">
                                                            <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" /> 终点
                                                        </div>
                                                        <div className="grid grid-cols-2 gap-x-2 gap-y-2.5">
                                                            <StatCell label="Pod" value={`${endSummary.readyPods}/${endSummary.pods}`} tone={endSummary.readyPods === endSummary.pods ? 'ok' : 'warn'} />
                                                            <StatCell label="Deploy" value={`${endSummary.readyDeployments}/${endSummary.deployments}`} tone={endSummary.readyDeployments === endSummary.deployments ? 'ok' : 'warn'} />
                                                            <StatCell label="节点" value={endSummary.nodes} />
                                                            <StatCell label="租户" value={endSummary.tenants} />
                                                        </div>
                                                    </div>
                                                </div>
                                            ) : (
                                                <div className="mt-2 text-[13px] text-[#5b6b82]">起点 / 终点快照缺失</div>
                                            )}
                                        </div>
                                        <div className="rounded-lg border border-white/[0.05] bg-panel-2/60 px-3 py-2.5">
                                            <div className="flex items-center gap-1.5 text-[12px] font-medium uppercase tracking-[0.1em] text-[#6b7a90]">
                                                <AlertTriangle className="h-3 w-3 text-[#5B8CFF]" />
                                                数据源新鲜度
                                            </div>
                                            <div className="mt-2 space-y-1.5">
                                                {Object.entries(segment.freshness).map(([name, state]) => (
                                                    <div key={name} className="flex items-center justify-between text-[13px]">
                                                        <span className="capitalize text-[#7d8ba0]">{name}</span>
                                                        <span className={`font-mono text-[12px] ${state.state === 'ready' ? 'text-emerald-300' : 'text-amber-200'}`}>
                                                            {state.state}
                                                        </span>
                                                    </div>
                                                ))}
                                            </div>
                                        </div>
                                    </div>
                                )
                            )}
                            {tab === 'decisions' && (
                                decisions.length === 0 ? (
                                    <div className="py-8 text-center text-[13px] text-[#5b6b82]">该窗口内没有配置决策</div>
                                ) : (
                                    <div className="divide-y divide-white/[0.04]">
                                        {decisions.map((snapshot) => (
                                            <button
                                                key={snapshot.id}
                                                type="button"
                                                onClick={() => selectSnapshot(snapshot.id)}
                                                className="flex w-full items-start gap-2.5 py-2 text-left hover:bg-white/[0.02]"
                                            >
                                                <span className="mt-1 h-1.5 w-1.5 shrink-0 rounded-full bg-[#5B8CFF]" />
                                                <span className="min-w-0 flex-1">
                                                    <span className="block truncate text-[12px] text-[#C9D6E8]">{snapshot.title}</span>
                                                    <span className="mt-0.5 block font-mono text-[12px] text-[#4f5f78]">{formatTime(snapshot.timestamp)}</span>
                                                </span>
                                                <span className="shrink-0 text-[12px] text-[#55657e]">{domainLabels[snapshot.domain] ?? snapshot.domain}</span>
                                            </button>
                                        ))}
                                    </div>
                                )
                            )}

                            {tab === 'metrics' && (
                                segment ? (
                                    <div className="space-y-3">
                                        {Object.entries(segment.metrics).map(([metricId, metric]) => {
                                            const stats = metricStats(metric)
                                            return (
                                                <div key={metricId} className="rounded-lg border border-white/[0.05] bg-panel-2/60 p-3">
                                                    <div className="flex items-baseline justify-between gap-2">
                                                        <span className="truncate font-mono text-[13px] text-[#9fb0c7]">{metricId}</span>
                                                        <span className="font-mono text-[14px] tabular-nums text-[#E2EAF4]">
                                                            {stats ? number.format(stats.latest) : '—'}
                                                            <span className="ml-1 text-[12px] text-[#52607a]">{metric.unit}</span>
                                                        </span>
                                                    </div>
                                                    <MetricMiniChart metric={metric} />
                                                    {stats && (
                                                        <div className="mt-1 grid grid-cols-4 gap-1.5 border-t border-white/[0.04] pt-2">
                                                            <StatCell label="min" value={number.format(stats.min)} />
                                                            <StatCell label="avg" value={number.format(stats.avg)} />
                                                            <StatCell label="max" value={number.format(stats.max)} />
                                                            <StatCell label="p95" value={number.format(stats.p95)} />
                                                        </div>
                                                    )}
                                                </div>
                                            )
                                        })}
                                    </div>
                                ) : (
                                    <div className="py-8 text-center text-[13px] text-[#5b6b82]">时间段指标不可用</div>
                                )
                            )}

                            {tab === 'anomalies' && (
                                anomalies.length === 0 ? (
                                    <div className="flex flex-col items-center gap-2 py-10 text-center">
                                        <CheckCircle2 className="h-5 w-5 text-emerald-400/70" />
                                        <div className="text-[12px] text-[#7d8ba0]">该窗口内没有异常</div>
                                        <div className="text-[13px] text-[#4f5f78]">所有切面均为正常状态</div>
                                    </div>
                                ) : (
                                    <div className="divide-y divide-white/[0.04]">
                                        {anomalies.map((snapshot) => {
                                            const meta = severityMeta[snapshot.severity] ?? severityMeta.attention
                                            return (
                                                <button
                                                    key={snapshot.id}
                                                    type="button"
                                                    onClick={() => selectSnapshot(snapshot.id)}
                                                    className="flex w-full items-start gap-2.5 py-2.5 text-left hover:bg-white/[0.02]"
                                                >
                                                    <span className={`mt-1 h-1.5 w-1.5 shrink-0 rounded-full ${meta.dot}`} />
                                                    <span className="min-w-0 flex-1">
                                                        <span className="block truncate text-[12px] text-[#D3DEEB]">{snapshot.title}</span>
                                                        <span className="mt-0.5 block font-mono text-[12px] text-[#4f5f78]">{formatTime(snapshot.timestamp)}</span>
                                                    </span>
                                                    <span className={`shrink-0 rounded-full border px-1.5 py-0.5 text-[12px] ${meta.pill}`}>{meta.label}</span>
                                                </button>
                                            )
                                        })}
                                    </div>
                                )
                            )}
                        </div>

                        <div className="flex items-center gap-2 border-t border-white/[0.05] bg-panel-2/40 px-3.5 py-2.5">
                            <button
                                type="button"
                                onClick={() => selectSnapshot(selected.id)}
                                className="flex h-7 items-center gap-1.5 rounded-lg border border-[#5B8CFF]/25 bg-[#5B8CFF]/[0.08] px-2.5 text-[13px] text-[#C8DBF5] transition-colors hover:bg-[#5B8CFF]/[0.15] hover:text-white"
                            >
                                <Crosshair className="h-3 w-3" />
                                时间轴跳转
                            </button>
                            <button
                                type="button"
                                onClick={copyId}
                                className="flex h-7 items-center gap-1.5 rounded-lg border border-white/[0.08] bg-transparent px-2.5 text-[13px] text-[#7d8ba0] transition-colors hover:bg-white/[0.04] hover:text-[#c3d0e0]"
                            >
                                <Copy className="h-3 w-3" />
                                {copied ? '已复制' : '复制 ID'}
                            </button>
                            <span className="ml-auto truncate font-mono text-[12px] text-[#47566d]">{selected.id}</span>
                        </div>
                    </>
                )}
            </aside>
        </div>
    )
}