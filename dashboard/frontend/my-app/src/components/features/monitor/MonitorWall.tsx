import { useMemo, useState } from 'react'
import { AlertTriangle, Gauge, RefreshCw } from 'lucide-react'
import ReactECharts from 'echarts-for-react'
import { useOverview } from '@/api/queries/traceQueries'
import { useReplayTimeContext } from '@/stores/timeSlice'
import { aggregateMetricPoints, metricStats } from '@/lib/formatters/metrics'
import type { MetricResult } from '@/types/trace.types'

type MetricStatus = 'ok' | 'warn' | 'crit' | 'unknown'

interface MetricSpec {
    id: string
    label: string
    aggregation: 'sum' | 'average'
    warnThreshold?: number
    critThreshold?: number
    format: (value: number) => string
}

const METRIC_SPECS: MetricSpec[] = [
    { id: 'simulator.ttft', label: 'TTFT 首字延迟', aggregation: 'average', warnThreshold: 2000, critThreshold: 5000, format: (v) => `${v.toFixed(0)} ms` },
    { id: 'simulator.qps', label: 'QPS 吞吐', aggregation: 'sum', format: (v) => `${v.toFixed(0)} req/s` },
    { id: 'simulator.queue', label: '队列积压', aggregation: 'sum', warnThreshold: 50, critThreshold: 200, format: (v) => `${v.toFixed(0)} req` },
    { id: 'simulator.tickLatency', label: 'Tick 延迟', aggregation: 'average', warnThreshold: 80, critThreshold: 200, format: (v) => `${v.toFixed(1)} ms` },
    { id: 'simulator.errorRate', label: '错误率', aggregation: 'average', warnThreshold: 0.02, critThreshold: 0.1, format: (v) => `${(v * 100).toFixed(2)}%` },
]

const statusMeta: Record<MetricStatus, { label: string; dot: string; block: string; text: string }> = {
    ok: { label: '正常', dot: 'bg-emerald-400', block: 'border-emerald-400/20 bg-emerald-400/[0.06]', text: 'text-emerald-300' },
    warn: { label: '告警', dot: 'bg-amber-300', block: 'border-amber-300/25 bg-amber-300/[0.08]', text: 'text-amber-200' },
    crit: { label: '严重', dot: 'bg-red-400', block: 'border-red-400/25 bg-red-400/[0.08]', text: 'text-red-300' },
    unknown: { label: '未知', dot: 'bg-[#748196]', block: 'border-white/[0.07] bg-white/[0.02]', text: 'text-[#8b99ad]' },
}

const number = new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 })

function statusFor(metric: MetricResult | undefined, spec: MetricSpec): MetricStatus {
    const stats = metric ? metricStats(metric) : null
    if (!stats) return 'unknown'
    const latest = stats.latest
    if (spec.critThreshold !== undefined && latest > spec.critThreshold) return 'crit'
    if (spec.warnThreshold !== undefined && latest > spec.warnThreshold) return 'warn'
    return 'ok'
}

function MetricCard({ spec, metric, onRetry }: {
    spec: MetricSpec
    metric: MetricResult | undefined
    onRetry: () => void
}) {
    const stats = metric ? metricStats(metric) : null
    const points = useMemo(() => (metric ? aggregateMetricPoints(metric, spec.aggregation) : []), [metric, spec.aggregation])
    const status = statusFor(metric, spec)
    const meta = statusMeta[status]
    const warnings = metric?.warnings ?? []

    const option = {
        animation: false,
        grid: { left: 2, right: 2, top: 6, bottom: 2 },
        xAxis: { type: 'time', show: false },
        yAxis: { type: 'value', show: false, scale: true },
        series: [{
            type: 'line',
            data: points.map((point) => [new Date(point.time).getTime(), point.value]),
            showSymbol: false,
            lineStyle: { color: '#5B8CFF', width: 1.4 },
            areaStyle: {
                color: {
                    type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
                    colorStops: [
                        { offset: 0, color: 'rgba(91,140,255,0.16)' },
                        { offset: 1, color: 'rgba(91,140,255,0)' },
                    ],
                },
            },
            markLine: {
                silent: true,
                symbol: 'none',
                data: [
                    ...(spec.warnThreshold !== undefined ? [{ yAxis: spec.warnThreshold, lineStyle: { color: 'rgba(246,183,60,.55)', width: 1, type: 'dashed' as const } }] : []),
                    ...(spec.critThreshold !== undefined ? [{ yAxis: spec.critThreshold, lineStyle: { color: 'rgba(239,91,103,.6)', width: 1, type: 'dashed' as const } }] : []),
                ],
            },
        }],
    }

    return (
        <div className="flex flex-col rounded-xl border border-white/[0.06] bg-panel/80 p-3.5">
            <div className="flex items-center gap-2">
                <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${meta.dot}`} />
                <span className="truncate text-[12px] font-medium text-[#C9D6E8]">{spec.label}</span>
                <span className="ml-auto font-mono text-[12px] text-[#52607a]">{spec.id.replace('simulator.', '')}</span>
            </div>
            <div className="mt-2 flex items-baseline gap-1.5">
                <span className={`font-mono text-[22px] font-semibold tabular-nums tracking-[-0.02em] ${meta.text}`}>
                    {stats ? number.format(stats.latest) : '—'}
                </span>
                <span className="text-[12px] text-[#64748d]">{stats ? spec.format(stats.latest) : '无数据'}</span>
            </div>
            <div className="mt-2 h-16">
                {points.length > 0 ? (
                    <ReactECharts option={option} style={{ height: 64 }} notMerge />
                ) : (
                    <div className="flex h-full items-center justify-center text-[12px] text-[#4f5f78]">暂无采样</div>
                )}
            </div>
            {stats && (
                <div className="mt-2.5 grid grid-cols-4 gap-1.5 border-t border-white/[0.045] pt-2">
                    {(['min', 'avg', 'max', 'p95'] as const).map((key) => (
                        <div key={key} className="min-w-0">
                            <div className="text-[12px] uppercase tracking-[0.08em] text-[#52607a]">{key}</div>
                            <div className="mt-0.5 truncate font-mono text-[12px] tabular-nums text-[#B7C4D6]">{number.format(stats[key])}</div>
                        </div>
                    ))}
                </div>
            )}
            {warnings.length > 0 && (
                <div className="mt-2.5 flex items-start gap-1.5 rounded-lg border border-amber-300/15 bg-amber-300/[0.045] px-2 py-1.5">
                    <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0 text-amber-300/80" />
                    <div className="min-w-0 flex-1">
                        <div className="text-[12px] leading-[15px] text-amber-100/80">{warnings.join('；')}</div>
                        <button type="button" onClick={onRetry} className="mt-1 flex items-center gap-1 text-[13px] text-amber-200/90 hover:text-white">
                            <RefreshCw className="h-2.5 w-2.5" /> 重试
                        </button>
                    </div>
                </div>
            )}
        </div>
    )
}

export function MonitorWall() {
    const replay = useReplayTimeContext()
    const query = useOverview(replay)
    const overview = query.data?.data
    const [statusFilter, setStatusFilter] = useState<MetricStatus | 'all'>('all')

    const metrics = useMemo(() => overview?.metrics ?? {}, [overview])
    const cards = useMemo(() => METRIC_SPECS.map((spec) => ({
        spec,
        metric: metrics[spec.id],
        status: statusFor(metrics[spec.id], spec),
    })), [metrics])

    const counts = useMemo(() => {
        const result: Record<MetricStatus, number> = { ok: 0, warn: 0, crit: 0, unknown: 0 }
        cards.forEach((card) => { result[card.status] += 1 })
        return result
    }, [cards])

    const visible = statusFilter === 'all' ? cards : cards.filter((card) => card.status === statusFilter)

    const filters: Array<{ key: MetricStatus | 'all'; label: string; count: number; dot: string; block: string; text: string }> = [
        { key: 'all', label: '全部', count: cards.length, dot: 'bg-[#5B8CFF]', block: 'border-white/[0.08] bg-white/[0.02]', text: 'text-[#AEBBD0]' },
        { key: 'ok', count: counts.ok, ...statusMeta.ok },
        { key: 'warn', count: counts.warn, ...statusMeta.warn },
        { key: 'crit', count: counts.crit, ...statusMeta.crit },
        { key: 'unknown', count: counts.unknown, ...statusMeta.unknown },
    ]

    if (query.isPending) {
        return (
            <div className="flex min-h-[320px] items-center justify-center gap-2 rounded-xl border border-white/[0.06] bg-panel/80 text-[13px] text-[#66758A]">
                <RefreshCw className="h-4 w-4 animate-spin text-[#6EA3F8]" />
                正在读取指标…
            </div>
        )
    }

    if (query.isError) {
        return (
            <div className="rounded-xl border border-red-400/15 bg-red-400/[0.04] p-5">
                <div className="flex items-start gap-3">
                    <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-red-300" />
                    <div className="min-w-0">
                        <div className="text-[12px] font-medium text-red-100">指标读取失败</div>
                        <p className="mt-1 break-words text-[13px] leading-5 text-red-100/60">{query.error.message}</p>
                        <button
                            type="button"
                            onClick={() => void query.refetch()}
                            className="mt-3 flex h-7 items-center gap-1.5 rounded-lg border border-red-300/15 bg-transparent px-3 text-[12px] text-red-100 hover:bg-red-300/[0.07]"
                        >
                            <RefreshCw className="h-3 w-3" />
                            重试
                        </button>
                    </div>
                </div>
            </div>
        )
    }

    return (
        <div className="space-y-3">
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-5">
                {filters.map((filter) => {
                    const active = statusFilter === filter.key
                    return (
                        <button
                            key={filter.key}
                            type="button"
                            onClick={() => setStatusFilter(active ? 'all' : filter.key)}
                            className={`flex items-center gap-2 rounded-xl border px-3 py-2.5 text-left transition-colors ${filter.block} ${
                                active ? 'ring-1 ring-[#5B8CFF]/50' : 'hover:bg-white/[0.03]'
                            }`}
                        >
                            <span className={`h-2 w-2 shrink-0 rounded-full ${filter.dot}`} />
                            <span className="text-[13px] text-[#AEBBD0]">{filter.label}</span>
                            <span className={`ml-auto font-mono text-[14px] font-semibold tabular-nums ${filter.text}`}>{filter.count}</span>
                        </button>
                    )
                })}
            </div>

            <div className="flex items-center gap-2 text-[12px] text-[#52607a]">
                <Gauge className="h-3.5 w-3.5 text-[#5B8CFF]" />
                {statusFilter === 'all' ? '全部指标' : `仅显示「${statusMeta[statusFilter].label}」`}
                <span className="ml-auto font-mono">{overview?.asOf ? new Date(overview.asOf).toLocaleString('zh-CN', { hour12: false }) : ''}</span>
            </div>

            {visible.length === 0 ? (
                <div className="rounded-xl border border-white/[0.06] bg-panel/80 px-6 py-10 text-center text-[13px] text-[#5b6b82]">
                    当前状态下没有指标卡片
                </div>
            ) : (
                <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                    {visible.map(({ spec, metric }) => (
                        <MetricCard key={spec.id} spec={spec} metric={metric} onRetry={() => void query.refetch()} />
                    ))}
                </div>
            )}
        </div>
    )
}