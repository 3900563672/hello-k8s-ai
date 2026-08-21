import { useMemo, useState } from 'react'
import {
    Activity,
    AlertTriangle,
    ArrowRight,
    Box,
    CheckCircle2,
    Clock3,
    GitBranch,
    Network,
    RefreshCw,
    Server,
} from 'lucide-react'
import ReactECharts from 'echarts-for-react'
import { Button } from '@/components/ui/button'
import { useSegment } from '@/api/queries/traceQueries'
import { useTimeStore } from '@/stores/timeSlice'
import type {
    MetricPoint,
    MetricResult,
    SegmentQuery,
    SegmentSnapshotData,
    TraceSummary,
} from '@/types/trace.types'

const segmentMetricCards = [
    { id: 'simulator.ttft', label: 'TTFT', aggregation: 'average' as const },
    { id: 'simulator.queue', label: 'Queue', aggregation: 'sum' as const },
    { id: 'simulator.qps', label: 'QPS', aggregation: 'sum' as const },
    { id: 'simulator.errorRate', label: 'Error Rate', aggregation: 'average' as const },
    { id: 'simulator.tickLatency', label: 'Latency P95', aggregation: 'average' as const },
]

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

function aggregateMetricPoints(
    metric: MetricResult,
    aggregation: 'sum' | 'average',
): MetricPoint[] {
    const buckets = new Map<number, { total: number; count: number }>()
    metric.series.forEach((series) => {
        series.points.forEach((point) => {
            const timestamp = new Date(point.time).getTime()
            if (!Number.isFinite(timestamp) || !Number.isFinite(point.value)) return
            const current = buckets.get(timestamp) ?? { total: 0, count: 0 }
            current.total += point.value
            current.count += 1
            buckets.set(timestamp, current)
        })
    })
    return [...buckets.entries()]
        .sort(([left], [right]) => left - right)
        .map(([timestamp, bucket]) => ({
            time: new Date(timestamp).toISOString(),
            value: aggregation === 'average'
                ? bucket.total / bucket.count
                : bucket.total,
        }))
}

function snapshotSummary(snapshot: SegmentSnapshotData | undefined) {
    if (!snapshot) return null
    const configuration = snapshot.configuration
    const workloads = snapshot.workloads
    const readyPods = workloads.pods.filter((pod) => pod.ready).length
    const readyDeployments = workloads.deployments.filter(
        (deployment) => deployment.readyReplicas >= deployment.desiredReplicas,
    ).length
    const totalReplicas = configuration.simulatorInstances.reduce((sum, instance) => {
        const replicas = typeof instance.spec?.replicas === 'number' ? instance.spec.replicas : 0
        return sum + replicas
    }, 0)
    const totalQPS = snapshot.traffic.tenants.reduce(
        (sum, tenant) => sum + (tenant.allocatedQPS ?? 0),
        0,
    )
    return {
        tenants: configuration.tenants.length,
        models: configuration.models.length,
        nodes: configuration.workerNodes.length,
        simulators: configuration.simulatorInstances.length,
        pods: `${readyPods}/${workloads.pods.length}`,
        deployments: `${readyDeployments}/${workloads.deployments.length}`,
        replicas: totalReplicas,
        qps: totalQPS,
    }
}

const comparisonRows: Array<{
    key: keyof NonNullable<ReturnType<typeof snapshotSummary>>
    label: string
    icon: typeof Activity
}> = [
    { key: 'tenants', label: 'Tenant', icon: Network },
    { key: 'models', label: 'Model', icon: Box },
    { key: 'nodes', label: 'WorkerNode', icon: Server },
    { key: 'simulators', label: 'Simulator', icon: Activity },
    { key: 'replicas', label: '副本总数', icon: Box },
    { key: 'qps', label: 'QPS', icon: Activity },
    { key: 'pods', label: 'Pod Ready', icon: CheckCircle2 },
    { key: 'deployments', label: 'Deployment Ready', icon: CheckCircle2 },
]

function MetricCurve({ label, metric, aggregation }: {
    label: string
    metric?: MetricResult
    aggregation: 'sum' | 'average'
}) {
    const points = metric ? aggregateMetricPoints(metric, aggregation) : []
    const latest = points.at(-1)?.value
    const option = {
        animation: false,
        grid: { left: 4, right: 8, top: 10, bottom: 2 },
        tooltip: {
            trigger: 'axis',
            backgroundColor: '#111722',
            borderColor: 'rgba(255,255,255,0.08)',
            padding: 8,
            textStyle: { color: '#DDE5F0', fontSize: 9 },
            axisPointer: { lineStyle: { color: 'rgba(91,140,255,0.35)' } },
        },
        xAxis: {
            type: 'time',
            axisLine: { lineStyle: { color: 'rgba(255,255,255,0.07)' } },
            axisTick: { show: false },
            axisLabel: { color: '#5E6D81', fontSize: 8, hideOverlap: true },
            splitLine: { show: false },
        },
        yAxis: {
            type: 'value',
            scale: true,
            axisLabel: { color: '#5E6D81', fontSize: 8 },
            splitLine: { lineStyle: { color: 'rgba(255,255,255,0.045)' } },
        },
        series: [
            {
                type: 'line',
                data: points.map((point) => [new Date(point.time).getTime(), point.value]),
                showSymbol: false,
                lineStyle: { color: '#689EF7', width: 1.4 },
                itemStyle: { color: '#689EF7' },
                areaStyle: {
                    color: {
                        type: 'linear',
                        x: 0,
                        y: 0,
                        x2: 0,
                        y2: 1,
                        colorStops: [
                            { offset: 0, color: 'rgba(104,158,247,0.16)' },
                            { offset: 1, color: 'rgba(104,158,247,0)' },
                        ],
                    },
                },
            },
        ],
    }
    return (
        <div className="rounded-xl border border-white/[0.07] bg-[#0A0E15]/90 p-3.5">
            <div className="flex items-center justify-between">
                <span className="text-[12px] font-medium text-[#9AA8BC]">{label}</span>
                <span className="font-mono text-[12px] text-[#D7E1ED]">
                    {latest === undefined ? '—' : number.format(latest)}
                </span>
            </div>
            <ReactECharts option={option} style={{ height: 96 }} notMerge />
        </div>
    )
}

function TraceRow({ trace }: { trace: TraceSummary }) {
    return (
        <div className="flex items-start justify-between gap-3 border-b border-white/[0.04] px-3.5 py-2.5 last:border-0">
            <div className="min-w-0">
                <div className="truncate text-[12px] font-medium text-[#D8E2EF]">
                    {trace.rootService} · {trace.rootOperation}
                </div>
                <div className="mt-1 truncate font-mono text-[12px] text-[#506077]">{trace.traceId}</div>
            </div>
            <div className="shrink-0 text-right text-[13px] text-[#68768A]">
                <div>{number.format(trace.durationMs)} ms · {trace.spanCount} spans</div>
                <div className="mt-0.5">{formatTime(trace.startTime)}</div>
            </div>
        </div>
    )
}

export function SegmentPanel() {
    const snapshots = useTimeStore((state) => state.snapshots)
    const [startId, setStartId] = useState<string>('')
    const [endId, setEndId] = useState<string>('')
    const [query, setQuery] = useState<SegmentQuery | null>(null)
    const result = useSegment(query)
    const segment = result.data?.data

    const ordered = useMemo(() => [...snapshots].sort(
        (left, right) => new Date(left.timestamp).getTime() - new Date(right.timestamp).getTime(),
    ), [snapshots])
    const startIndex = ordered.findIndex((item) => item.id === startId)
    const endIndex = ordered.findIndex((item) => item.id === endId)
    const start = startIndex >= 0 ? ordered[startIndex] : null
    const end = endIndex >= 0 ? ordered[endIndex] : null
    const valid = Boolean(start && end && startIndex < endIndex)

    const runAnalysis = () => {
        if (!start || !end || startIndex >= endIndex) return
        setQuery({
            start: start.timestamp,
            end: end.timestamp,
        })
    }

    const startSummary = snapshotSummary(segment?.startSnapshot)
    const endSummary = snapshotSummary(segment?.endSnapshot)
    const durationMs = segment ? new Date(segment.end).getTime() - new Date(segment.start).getTime() : 0
    const durationLabel = durationMs >= 3_600_000
        ? `${(durationMs / 3_600_000).toFixed(1)} 小时`
        : `${Math.max(1, Math.round(durationMs / 60_000))} 分钟`

    return (
        <section className="rounded-xl border border-[#5B8CFF]/12 bg-[#0A0E15]/70">
            <div className="flex flex-col gap-3 border-b border-white/[0.06] p-4 lg:flex-row lg:items-center lg:justify-between">
                <div>
                    <div className="flex items-center gap-2 text-[12px] font-medium text-[#9AA8BC]">
                        <Clock3 className="h-3.5 w-3.5 text-[#7CAEFF]" />
                        时间段切面（Run Segment）
                        <span className="rounded-full border border-[#9EAEFF]/20 bg-[#9EAEFF]/[0.07] px-2 py-0.5 text-[12px] text-[#AEB9FF]">
                            起点 + 终点快照 + 区间指标 / Trace
                        </span>
                    </div>
                    <p className="mt-1 text-[13px] text-[#657286]">
                        选择起点与终点快照，分析一次调度/实验从什么状态开始、到什么状态结束、中间发生了什么
                    </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                    <select
                        value={startId}
                        onChange={(event) => setStartId(event.target.value)}
                        className="h-8 rounded-lg border border-white/[0.08] bg-[#0D1420] px-2 text-[12px] text-[#C7D3E2] outline-none focus:border-[#5B8CFF]/40"
                    >
                        <option value="">起点快照…</option>
                        {ordered.slice(0, Math.max(1, ordered.length - 1)).map((item) => (
                            <option key={item.id} value={item.id}>
                                {formatTime(item.timestamp)} · {item.title}
                            </option>
                        ))}
                    </select>
                    <ArrowRight className="h-3.5 w-3.5 text-[#465267]" />
                    <select
                        value={endId}
                        onChange={(event) => setEndId(event.target.value)}
                        className="h-8 rounded-lg border border-white/[0.08] bg-[#0D1420] px-2 text-[12px] text-[#C7D3E2] outline-none focus:border-[#5B8CFF]/40"
                    >
                        <option value="">终点快照…</option>
                        {ordered.slice(Math.max(0, startIndex + 1)).map((item) => (
                            <option key={item.id} value={item.id}>
                                {formatTime(item.timestamp)} · {item.title}
                            </option>
                        ))}
                    </select>
                    <Button
                        type="button"
                        variant="outline"
                        disabled={!valid || result.isFetching}
                        onClick={runAnalysis}
                        className="h-8 gap-2 border-[#5B8CFF]/25 bg-[#5B8CFF]/[0.06] px-3 text-[12px] text-[#AFCBFF] hover:bg-[#5B8CFF]/[0.12] hover:text-white"
                    >
                        <GitBranch className="h-3.5 w-3.5" />
                        分析时间段
                    </Button>
                </div>
            </div>

            {ordered.length === 0 && (
                <div className="px-4 py-6 text-center text-[13px] text-[#536177]">
                    时间轴暂无快照——运行流量后即可使用段分析
                </div>
            )}

            {!query && ordered.length > 0 && (
                <div className="px-4 py-6 text-center text-[13px] text-[#536177]">
                    选择一个时间段后查看起点 → 终点状态对比、区间指标与 Trace
                </div>
            )}

            {result.isPending && query && (
                <div className="flex items-center justify-center gap-2 px-4 py-10 text-[12px] text-[#66758A]">
                    <RefreshCw className="h-4 w-4 animate-spin text-[#6EA3F8]" />
                    正在聚合段数据…
                </div>
            )}
            {result.isError && query && (
                <div className="px-4 py-5 text-[13px] leading-4 text-red-300">
                    Segment API 请求失败：{result.error.message}
                </div>
            )}

            {segment && (
                <div className="space-y-4 p-4">
                    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[13px] text-[#68768A]">
                        <span>
                            <span className="text-[#9AA8BC]">起点</span>{' '}
                            {formatTime(segment.startSnapshot?.capturedAt ?? segment.start)}
                        </span>
                        <ArrowRight className="h-3 w-3" />
                        <span>
                            <span className="text-[#9AA8BC]">终点</span>{' '}
                            {formatTime(segment.endSnapshot?.capturedAt ?? segment.end)}
                        </span>
                        <span>时长 {durationLabel}</span>
                        {segment.startSnapshot?.snapshotId && (
                            <span className="font-mono text-[12px]">
                                {segment.startSnapshot.snapshotId} → {segment.endSnapshot?.snapshotId}
                            </span>
                        )}
                    </div>

                    {segment.availability !== 'available' && (
                        <div className="flex items-start gap-2 rounded-lg border border-amber-300/10 bg-amber-300/[0.035] px-3 py-2 text-[13px] leading-4 text-amber-100/75">
                            <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0 text-amber-300/70" />
                            起点或终点之前没有持久化快照，无法构成完整段切面。
                        </div>
                    )}
                    {(result.data?.meta.warnings ?? []).map((warning) => (
                        <div key={warning} className="flex items-start gap-2 rounded-lg border border-amber-300/10 bg-amber-300/[0.035] px-3 py-2 text-[13px] leading-4 text-amber-100/75">
                            <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0 text-amber-300/70" />
                            {warning}
                        </div>
                    ))}

                    {startSummary && endSummary && (
                        <div className="grid gap-2">
                            <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
                                <span className="text-[12px] uppercase tracking-[0.1em] text-[#607086]">起点状态</span>
                                <ArrowRight className="h-3 w-3 text-[#465267]" />
                                <span className="text-right text-[12px] uppercase tracking-[0.1em] text-[#607086]">终点状态</span>
                            </div>
                            <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
                                {comparisonRows.map((row) => {
                                    const before = startSummary[row.key]
                                    const after = endSummary[row.key]
                                    const changed = before !== after
                                    return (
                                        <div key={row.key} className="rounded-xl border border-white/[0.07] bg-[#0A0E15]/90 p-3">
                                            <div className="flex items-center justify-between">
                                                <span className="text-[13px] uppercase tracking-[0.08em] text-[#607086]">{row.label}</span>
                                                <row.icon className="h-3 w-3 text-[#6EA3F8]" />
                                            </div>
                                            <div className="mt-2 flex items-baseline gap-1.5">
                                                <span className="font-mono text-sm text-[#AAB6C8]">{before}</span>
                                                <ArrowRight className="h-3 w-3 text-[#465267]" />
                                                <span className={`font-mono text-sm ${changed ? 'text-[#8FD0A8]' : 'text-[#E9F1FB]'}`}>{after}</span>
                                            </div>
                                        </div>
                                    )
                                })}
                            </div>
                        </div>
                    )}

                    <div>
                        <div className="mb-2 text-[12px] font-medium text-[#9AA8BC]">
                            区间指标 <span className="text-[12px] font-normal text-[#536177]">Prometheus · 整个时间段</span>
                        </div>
                        <div className="grid gap-2.5 md:grid-cols-2 xl:grid-cols-3">
                            {segmentMetricCards.map((card) => (
                                <MetricCurve
                                    key={card.id}
                                    label={card.label}
                                    metric={segment.metrics[card.id]}
                                    aggregation={card.aggregation}
                                />
                            ))}
                        </div>
                    </div>

                    <div>
                        <div className="mb-2 text-[12px] font-medium text-[#9AA8BC]">
                            区间 Trace <span className="text-[12px] font-normal text-[#536177]">Jaeger · {segment.traces.length} 条</span>
                        </div>
                        {segment.traces.length === 0 ? (
                            <div className="rounded-xl border border-white/[0.06] px-4 py-6 text-center text-[13px] text-[#536177]">
                                该时间段内没有 Trace
                            </div>
                        ) : (
                            <div className="overflow-hidden rounded-xl border border-white/[0.06] bg-[#0A0E15]/70">
                                {segment.traces.slice(0, 50).map((trace) => (
                                    <TraceRow key={trace.traceId} trace={trace} />
                                ))}
                            </div>
                        )}
                    </div>
                </div>
            )}
        </section>
    )
}
