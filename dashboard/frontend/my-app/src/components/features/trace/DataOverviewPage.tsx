import { useState } from 'react'
import {
    Activity,
    AlertTriangle,
    Box,
    Boxes,
    CheckCircle2,
    Clock3,
    Database,
    GitBranch,
    History,
    Network,
    RefreshCw,
    Server,
    TimerReset,
} from 'lucide-react'
import ReactECharts from 'echarts-for-react'
import { Button } from '@/components/ui/button'
import { useOverview, useTraceDetail } from '@/api/queries/traceQueries'
import { useReplayTimeContext, useTimeStore } from '@/stores/timeSlice'
import { SegmentPanel } from '@/components/features/trace/SegmentPanel'
import { ExperimentPanel } from '@/components/features/trace/ExperimentPanel'
import type {
    BackendDeployment,
    BackendEvent,
    BackendPod,
    MetricPoint,
    MetricResult,
    OverviewData,
    ProviderState,
    ResourceRef,
    TraceSpan,
} from '@/types/trace.types'
import type { BackendResource, KubernetesCondition } from '@/types/config.types'

const metricCards = [
    { id: 'simulator.ttft', label: 'TTFT', aggregation: 'average' },
    { id: 'simulator.queue', label: 'Queue', aggregation: 'sum' },
    { id: 'simulator.qps', label: 'QPS', aggregation: 'sum' },
    { id: 'simulator.errorRate', label: 'Error Rate', aggregation: 'average' },
    { id: 'simulator.tickLatency', label: 'Latency P95', aggregation: 'average' },
    { id: 'simulator.timeScale', label: 'Time Scale', aggregation: 'average' },
] as const

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

function relativeAge(value?: string, baseline?: string) {
    if (!value) return '未知'
    const baselineTime = baseline ? new Date(baseline).getTime() : Date.now()
    const age = baselineTime - new Date(value).getTime()
    if (!Number.isFinite(age)) return '未知'
    if (age < 1_000) return '< 1 秒'
    if (age < 60_000) return `${Math.floor(age / 1_000)} 秒`
    if (age < 3_600_000) return `${Math.floor(age / 60_000)} 分钟`
    return `${Math.floor(age / 3_600_000)} 小时`
}

function resourcePhase(resource: BackendResource) {
    const phase = resource.status.phase
    if (typeof phase === 'string' && phase) return phase
    const ready = resource.conditions.find((condition) => condition.type === 'Ready')
    if (ready) return ready.status === 'True' ? 'Ready' : ready.reason || ready.status
    return Object.keys(resource.status).length > 0 ? 'Observed' : 'Pending'
}

function latestMetricValues(metric?: MetricResult) {
    if (!metric) return []
    return metric.series.flatMap((series) => {
        const point = series.points.at(-1)
        return point && Number.isFinite(point.value) ? [point.value] : []
    })
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

function metricDisplay(
    metric: MetricResult | undefined,
    aggregation: 'sum' | 'average',
) {
    const values = latestMetricValues(metric)
    if (values.length === 0 || !metric) return { value: '—', unit: '', points: [] }
    const total = values.reduce((sum, value) => sum + value, 0)
    const value = aggregation === 'average' ? total / values.length : total
    const displayValue = metric.unit === 'ratio'
        ? `${number.format(value * 100)}%`
        : number.format(value)
    return {
        value: displayValue,
        unit: metric.unit === 'ratio' ? '' : metric.unit,
        points: aggregateMetricPoints(metric, aggregation),
    }
}

export function DataOverviewPage() {
    const replay = useReplayTimeContext()
    const returnToLatest = useTimeStore((state) => state.returnToLatest)
    const query = useOverview(replay)
    const overview = query.data?.data

    return (
        <div className="relative h-full overflow-auto bg-[#05070A] text-[#E8EEF7]">
            <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_56%_6%,rgba(91,140,255,.08),transparent_28%)]" />
            <main className="relative mx-auto w-full max-w-[1500px] px-5 py-6 lg:px-8 lg:py-8">
                <header className="flex flex-col gap-4 border-b border-white/[0.07] pb-5 lg:flex-row lg:items-end lg:justify-between">
                    <div>
                        <div className="flex items-center gap-2 text-[10px] font-medium uppercase tracking-[0.16em] text-[#6B788C]">
                            <Database className="h-3.5 w-3.5 text-[#7CAEFF]" />
                            Data replay / overview
                            <SourceBadge historical={replay.mode === 'historical'} />
                        </div>
                        <h1 className="mt-3 text-2xl font-semibold tracking-[-0.025em] text-[#F0F5FB]">
                            Kubernetes 真实状态回显
                        </h1>
                        <p className="mt-1.5 text-[11px] text-[#657286]">
                            Informer cache、Prometheus、Jaeger 与 PostgreSQL 时间切面的统一读模型
                        </p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                        {replay.mode === 'historical' && (
                            <Button
                                type="button"
                                variant="outline"
                                onClick={returnToLatest}
                                className="h-8 gap-2 border-[#5B8CFF]/25 bg-[#5B8CFF]/[0.06] px-3 text-[10px] text-[#AFCBFF] hover:bg-[#5B8CFF]/[0.12] hover:text-white"
                            >
                                <History className="h-3.5 w-3.5" />
                                回到最新状态
                            </Button>
                        )}
                        <Button
                            type="button"
                            variant="outline"
                            disabled={query.isFetching}
                            onClick={() => void query.refetch()}
                            className="h-8 gap-2 border-white/[0.08] bg-white/[0.025] px-3 text-[10px] text-[#AAB6C8] hover:bg-white/[0.06] hover:text-white"
                        >
                            <RefreshCw className={`h-3.5 w-3.5 ${query.isFetching ? 'animate-spin' : ''}`} />
                            刷新
                        </Button>
                    </div>
                </header>

                <SegmentPanel />
                <ExperimentPanel />

                {query.isPending && <LoadingState />}
                {query.isError && <ErrorState message={query.error.message} retry={() => void query.refetch()} />}
                {overview && (
                    <OverviewContent
                        overview={overview}
                        warnings={query.data?.meta.warnings ?? []}
                    />
                )}
            </main>
        </div>
    )
}

function OverviewContent({ overview, warnings }: { overview: OverviewData; warnings: string[] }) {
    const [selectedTraceId, setSelectedTraceId] = useState<string | null>(null)
    const traceDetail = useTraceDetail(selectedTraceId)
    const configuration = overview.configuration
    const workloads = overview.workloads
    const readyPods = workloads.pods.filter((pod) => pod.ready).length
    const readyDeployments = workloads.deployments.filter(
        (deployment) => deployment.readyReplicas >= deployment.desiredReplicas,
    ).length
    const currentStatus = [
        { label: 'Tenant', value: configuration.tenants.length, icon: Network },
        { label: 'Model', value: configuration.models.length, icon: Box },
        { label: 'WorkerNode', value: configuration.workerNodes.length, icon: Server },
        { label: 'Simulator', value: configuration.simulatorInstances.length, icon: Activity },
        { label: 'Pod Ready', value: `${readyPods}/${workloads.pods.length}`, icon: Boxes },
        {
            label: 'Deployment Ready',
            value: `${readyDeployments}/${workloads.deployments.length}`,
            icon: CheckCircle2,
        },
    ]

    return (
        <div className="space-y-5 pt-5">
            <ClockPanel overview={overview} />

            {overview.availability !== 'available' && (
                <Notice tone="warning" text="该时间点没有可用的 PostgreSQL Kubernetes 快照。" />
            )}
            {warnings.length > 0 && (
                <div className="grid gap-2">
                    {warnings.map((warning) => <Notice key={warning} tone="warning" text={warning} />)}
                </div>
            )}

            <section className="grid grid-cols-2 gap-2.5 md:grid-cols-3 xl:grid-cols-6">
                {currentStatus.map((item) => (
                    <div key={item.label} className="rounded-xl border border-white/[0.07] bg-[#0A0E15]/90 p-3.5">
                        <div className="flex items-center justify-between">
                            <span className="text-[9px] uppercase tracking-[0.1em] text-[#607086]">{item.label}</span>
                            <item.icon className="h-3.5 w-3.5 text-[#6EA3F8]" />
                        </div>
                        <div className="mt-3 font-mono text-xl font-medium text-[#E9F1FB]">{item.value}</div>
                    </div>
                ))}
            </section>

            <section>
                <SectionTitle icon={Activity} title="性能数据" subtitle="Prometheus · 最近 15 分钟" />
                <div className="mt-3 grid gap-2.5 md:grid-cols-2 xl:grid-cols-6">
                    {metricCards.map((card) => {
                        const metric = overview.metrics[card.id]
                        const display = metricDisplay(metric, card.aggregation)
                        return (
                            <div key={card.id} className="overflow-hidden rounded-xl border border-white/[0.07] bg-[#0A0E15]/90 p-3.5">
                                <div className="flex items-center justify-between">
                                    <span className="text-[10px] font-medium text-[#9AA8BC]">{card.label}</span>
                                    <ProviderDot ready={Boolean(metric)} />
                                </div>
                                <div className="mt-2.5 flex items-baseline gap-1.5">
                                    <span className="font-mono text-xl text-[#EAF2FC]">{display.value}</span>
                                    <span className="text-[9px] text-[#5E6D81]">{display.unit}</span>
                                </div>
                                <Sparkline points={display.points} />
                            </div>
                        )
                    })}
                </div>
                <div className="mt-2.5 grid gap-2.5 md:grid-cols-2 xl:grid-cols-3">
                    {metricCards.map((card) => (
                        <MetricTrendChart
                            key={card.id}
                            label={card.label}
                            metric={overview.metrics[card.id]}
                            aggregation={card.aggregation}
                        />
                    ))}
                </div>
            </section>

            <section>
                <SectionTitle icon={Network} title="Traffic / Performance" subtitle="Tenant、SimulatorInstance 与 Pod 实际分配" />
                <TrafficTable tenants={overview.traffic.tenants} />
            </section>

            <section className="grid gap-4 xl:grid-cols-[minmax(0,1.4fr)_minmax(340px,.6fr)]">
                <div className="min-w-0">
                    <SectionTitle icon={GitBranch} title="Kubernetes 资源" subtitle="CRD 与工作负载关联状态" />
                    <div className="mt-3 space-y-3">
                        <ResourceTable
                            title="Tenant / Model / WorkerNode"
                            resources={[
                                ...configuration.tenants,
                                ...configuration.models,
                                ...configuration.workerNodes,
                            ]}
                        />
                        <ResourceTable title="SimulatorInstance" resources={configuration.simulatorInstances} />
                        <ResourceTable
                            title="Clock / Policy / Orchestrator / Performance / Runtime"
                            resources={[
                                ...(configuration.simulationClocks ?? []),
                                ...configuration.policies.tenantModel,
                                ...configuration.policies.tenantNode,
                                ...configuration.policies.modelNode,
                                ...configuration.orchestrators,
                                ...configuration.tenantPerformance,
                                ...configuration.tenantRuntimes,
                            ]}
                        />
                        <WorkloadTable pods={workloads.pods} deployments={workloads.deployments} />
                        <InfrastructureTable
                            nodes={workloads.nodes}
                            services={workloads.services}
                            leases={workloads.leases}
                        />
                    </div>
                </div>
                <div className="min-w-0">
                    <SectionTitle icon={GitBranch} title="Trace 调用链" subtitle="Jaeger · 最近 20 条" />
                    <div className="mt-3 rounded-xl border border-white/[0.07] bg-[#0A0E15]/90">
                        {overview.traces.length === 0 ? (
                            <EmptyRow text="当前窗口没有 Trace，或 Jaeger Provider 不可用" />
                        ) : (
                            <div className="divide-y divide-white/[0.055]">
                                {overview.traces.map((trace) => (
                                    <button
                                        type="button"
                                        key={trace.traceId}
                                        onClick={() => setSelectedTraceId(trace.traceId)}
                                        className={`block w-full p-3.5 text-left transition hover:bg-white/[0.025] ${selectedTraceId === trace.traceId ? 'bg-[#5B8CFF]/[0.055]' : ''}`}
                                    >
                                        <div className="flex items-start justify-between gap-3">
                                            <div className="min-w-0">
                                                <div className="truncate text-[11px] font-medium text-[#D8E2EF]">
                                                    {trace.rootService} · {trace.rootOperation}
                                                </div>
                                                <div className="mt-1 truncate font-mono text-[8px] text-[#506077]">{trace.traceId}</div>
                                            </div>
                                            <StatusPill
                                                label={trace.errorSpanCount > 0 ? `${trace.errorSpanCount} error` : 'OK'}
                                                ready={trace.errorSpanCount === 0}
                                            />
                                        </div>
                                        <div className="mt-2 flex gap-3 text-[9px] text-[#68768A]">
                                            <span>{number.format(trace.durationMs)} ms</span>
                                            <span>{trace.spanCount} spans</span>
                                            <span>{formatTime(trace.startTime)}</span>
                                        </div>
                                    </button>
                                ))}
                            </div>
                        )}
                    </div>
                    {selectedTraceId && (
                        <TraceDetailPanel
                            traceId={selectedTraceId}
                            loading={traceDetail.isPending}
                            error={traceDetail.error?.message}
                            spans={traceDetail.data?.data.spans ?? []}
                            links={traceDetail.data?.data.entityLinks ?? []}
                            close={() => setSelectedTraceId(null)}
                        />
                    )}
                </div>
            </section>

            <section className="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(360px,.65fr)]">
                <div className="min-w-0">
                    <SectionTitle icon={AlertTriangle} title="Kubernetes Events" subtitle={`最近 ${overview.workloads.events.length} 条`} />
                    <EventTable events={overview.workloads.events} />
                </div>
                <div className="min-w-0">
                    <SectionTitle icon={TimerReset} title="Data Freshness" subtitle="各数据源最后观测状态" />
                    <ProviderTable providers={overview.freshness} serverTime={overview.clock.serverTime} />
                </div>
            </section>
        </div>
    )
}

function TraceDetailPanel({
    traceId,
    loading,
    error,
    spans,
    links,
    close,
}: {
    traceId: string
    loading: boolean
    error?: string
    spans: TraceSpan[]
    links: ResourceRef[]
    close: () => void
}) {
    const parentBySpan = new Map(spans.map((span) => [span.spanId, span.parentSpanId]))
    const depthOf = (span: TraceSpan) => {
        let depth = 0
        let parent = span.parentSpanId
        const seen = new Set<string>()
        while (parent && parentBySpan.has(parent) && !seen.has(parent) && depth < 8) {
            seen.add(parent)
            parent = parentBySpan.get(parent)
            depth++
        }
        return depth
    }
    const ordered = [...spans].sort(
        (left, right) => new Date(left.startTime).getTime() - new Date(right.startTime).getTime(),
    )
    const traceStart = ordered.length > 0
        ? Math.min(...ordered.map((span) => new Date(span.startTime).getTime()))
        : 0
    const traceEnd = ordered.length > 0
        ? Math.max(...ordered.map((span) => new Date(span.startTime).getTime() + span.durationMs))
        : traceStart + 1
    const totalDuration = Math.max(traceEnd - traceStart, 1)
    return (
        <div className="mt-3 overflow-hidden rounded-xl border border-[#5B8CFF]/15 bg-[#090E17]">
            <div className="flex items-start justify-between gap-3 border-b border-white/[0.06] p-3.5">
                <div className="min-w-0">
                    <div className="text-[10px] font-medium text-[#C8D7EA]">Span 调用链</div>
                    <div className="mt-1 truncate font-mono text-[8px] text-[#52627A]">{traceId}</div>
                </div>
                <button type="button" onClick={close} className="text-[9px] text-[#68778B] hover:text-white">关闭</button>
            </div>
            {loading && <EmptyRow text="正在读取 Jaeger Span…" />}
            {error && <div className="p-3.5 text-[9px] leading-4 text-red-300">{error}</div>}
            {!loading && !error && ordered.length === 0 && <EmptyRow text="Trace 中没有 Span" />}
            {ordered.length > 0 && (
                <div className="max-h-[360px] overflow-auto divide-y divide-white/[0.045]">
                    {ordered.map((span) => {
                        const depth = depthOf(span)
                        return (
                            <div key={span.spanId} className="relative py-2.5 pr-3" style={{ paddingLeft: `${14 + depth * 13}px` }}>
                                {depth > 0 && <span className="absolute top-0 bottom-0 border-l border-[#5B8CFF]/15" style={{ left: `${8 + (depth - 1) * 13}px` }} />}
                                <div className="flex items-start justify-between gap-2">
                                    <div className="min-w-0">
                                        <div className="truncate text-[9px] text-[#C6D2E2]">{span.service} · {span.operation}</div>
                                        <div className="mt-1 truncate font-mono text-[7px] text-[#4E5E75]">{span.spanId}</div>
                                    </div>
                                    <StatusPill label={`${number.format(span.durationMs)} ms`} ready={span.status !== 'error'} />
                                </div>
                                <div className="relative mt-1.5 h-[7px] overflow-hidden rounded-full bg-white/[0.045]">
                                    <div
                                        className={`absolute top-0 h-full rounded-full ${span.status === 'error' ? 'bg-amber-400/60' : 'bg-[#5B8CFF]/[0.55]'}`}
                                        style={{
                                            left: `${((new Date(span.startTime).getTime() - traceStart) / totalDuration) * 100}%`,
                                            width: `${Math.max((span.durationMs / totalDuration) * 100, 1)}%`,
                                        }}
                                        title={`+${number.format(new Date(span.startTime).getTime() - traceStart)} ms · ${number.format(span.durationMs)} ms`}
                                    />
                                </div>
                                {span.events.length > 0 && (
                                    <div className="mt-1.5 text-[8px] text-[#65748A]">{span.events.length} span events</div>
                                )}
                            </div>
                        )
                    })}
                </div>
            )}
            {links.length > 0 && (
                <div className="flex flex-wrap gap-1.5 border-t border-white/[0.055] p-3">
                    {links.map((link) => (
                        <span key={`${link.kind}/${link.name}`} className="rounded border border-[#5B8CFF]/15 bg-[#5B8CFF]/[0.05] px-1.5 py-0.5 text-[8px] text-[#92B9F5]">
                            {link.kind}/{link.name}
                        </span>
                    ))}
                </div>
            )}
        </div>
    )
}

function ClockPanel({ overview }: { overview: OverviewData }) {
    const items = [
        { label: 'Server Time', value: overview.clock.serverTime },
        { label: 'Logical Time', value: overview.clock.logicalTime },
        { label: 'Observed Time', value: overview.asOf },
        { label: 'Data Freshness', value: `${relativeAge(overview.asOf, overview.clock.serverTime)}前` },
    ]
    return (
        <section className="grid overflow-hidden rounded-xl border border-white/[0.07] bg-[#090D14]/90 md:grid-cols-4">
            {items.map((item, index) => (
                <div key={item.label} className={`p-3.5 ${index > 0 ? 'border-t border-white/[0.06] md:border-l md:border-t-0' : ''}`}>
                    <div className="flex items-center gap-1.5 text-[9px] uppercase tracking-[0.1em] text-[#59687D]">
                        <Clock3 className="h-3 w-3 text-[#658DD0]" />
                        {item.label}
                    </div>
                    <div className="mt-2 font-mono text-[10px] text-[#BECBDD]">
                        {item.label === 'Data Freshness' ? item.value : formatTime(item.value)}
                    </div>
                </div>
            ))}
        </section>
    )
}

function TrafficTable({ tenants }: { tenants: OverviewData['traffic']['tenants'] }) {
    return (
        <div className="mt-3 overflow-hidden rounded-xl border border-white/[0.07] bg-[#0A0E15]/90">
            {tenants.length === 0 ? <EmptyRow text="Kubernetes 中没有 Tenant Traffic 读模型" /> : (
                <div className="overflow-x-auto">
                    <table className="w-full min-w-[900px] text-left">
                        <thead className="border-b border-white/[0.055] text-[8px] uppercase tracking-[0.12em] text-[#4F5D71]">
                            <tr>
                                <th className="px-3.5 py-2 font-medium">Tenant</th>
                                <th className="px-3.5 py-2 font-medium">Requested / Allocated</th>
                                <th className="px-3.5 py-2 font-medium">TTFT</th>
                                <th className="px-3.5 py-2 font-medium">Queue</th>
                                <th className="px-3.5 py-2 font-medium">Runtime</th>
                                <th className="px-3.5 py-2 font-medium">Instances</th>
                                <th className="px-3.5 py-2 font-medium">Ready Replicas</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-white/[0.045]">
                            {tenants.map((tenant) => (
                                <tr key={tenant.tenant.name} className="text-[10px] text-[#9BA9BC]">
                                    <td className="px-3.5 py-2.5">
                                        <div className="text-[#D7E1ED]">{tenant.displayName || tenant.tenant.name}</div>
                                        <div className="mt-0.5 font-mono text-[8px] text-[#536178]">{tenant.tenant.name} · {tenant.priority}</div>
                                    </td>
                                    <td className="px-3.5 py-2.5 font-mono">
                                        <span className="text-[#CFDAE8]">{tenant.requestedQPS}</span>
                                        <span className="mx-1.5 text-[#465267]">/</span>
                                        <span className={tenant.allocationBalanced ? 'text-emerald-300' : 'text-amber-200'}>{tenant.allocatedQPS}</span>
                                        <span className="ml-1 text-[8px] text-[#59687D]">QPS</span>
                                    </td>
                                    <td className="px-3.5 py-2.5 font-mono">{tenant.performance.avgTTFT ? `${number.format(tenant.performance.avgTTFT.value)} ${tenant.performance.avgTTFT.unit}` : '—'}</td>
                                    <td className="px-3.5 py-2.5 font-mono">{tenant.performance.avgQueue ? `${number.format(tenant.performance.avgQueue.value)} ${tenant.performance.avgQueue.unit}` : '—'}</td>
                                    <td className="px-3.5 py-2.5"><StatusPill label={tenant.runtimePhase || tenant.performance.phase || 'Unknown'} ready={tenant.runtimePhase === 'Running' || tenant.performance.phase === 'Running'} /></td>
                                    <td className="px-3.5 py-2.5">
                                        <div className="flex max-w-[280px] flex-wrap gap-1">
                                            {tenant.instances.length === 0 ? <span className="text-[#4F5D71]">None</span> : tenant.instances.map((instance) => (
                                                <span key={instance.name} title={`${instance.model} · ${instance.assignedQPS} QPS · ${instance.pods.length} Pods`} className="rounded border border-[#5B8CFF]/15 bg-[#5B8CFF]/[0.045] px-1.5 py-0.5 font-mono text-[8px] text-[#92B9F5]">
                                                    {instance.name} · {instance.availableReplicas}/{instance.desiredReplicas}
                                                </span>
                                            ))}
                                        </div>
                                    </td>
                                    <td className="px-3.5 py-2.5 font-mono text-[#D5DFEC]">{tenant.readyReplicaCount}</td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    )
}

function ResourceTable({ title, resources }: { title: string; resources: BackendResource[] }) {
    return (
        <div className="overflow-hidden rounded-xl border border-white/[0.07] bg-[#0A0E15]/90">
            <div className="flex items-center justify-between border-b border-white/[0.06] px-3.5 py-2.5">
                <span className="text-[10px] font-medium text-[#98A7BB]">{title}</span>
                <span className="font-mono text-[9px] text-[#546277]">{resources.length}</span>
            </div>
            {resources.length === 0 ? <EmptyRow text="Informer cache 中没有该类资源" /> : (
                <div className="overflow-x-auto">
                    <table className="w-full min-w-[620px] text-left">
                        <thead className="border-b border-white/[0.055] text-[8px] uppercase tracking-[0.12em] text-[#4F5D71]">
                            <tr>
                                <th className="px-3.5 py-2 font-medium">Kind / Name</th>
                                <th className="px-3.5 py-2 font-medium">Phase</th>
                                <th className="px-3.5 py-2 font-medium">Conditions</th>
                                <th className="px-3.5 py-2 font-medium">Generation</th>
                                <th className="px-3.5 py-2 font-medium">Resource Version</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-white/[0.045]">
                            {resources.map((resource) => (
                                <tr key={`${resource.ref.kind}/${resource.ref.name}`} className="text-[10px] text-[#9BA9BC]">
                                    <td className="px-3.5 py-2.5">
                                        <div className="text-[#D7E1ED]">{resource.ref.name}</div>
                                        <div className="mt-0.5 text-[8px] text-[#536178]">{resource.ref.kind}</div>
                                    </td>
                                    <td className="px-3.5 py-2.5"><StatusPill label={resourcePhase(resource)} ready={isResourceReady(resource)} /></td>
                                    <td className="max-w-[280px] px-3.5 py-2.5"><ConditionSummary conditions={resource.conditions} /></td>
                                    <td className="px-3.5 py-2.5 font-mono">{resource.metadata.generation}</td>
                                    <td className="px-3.5 py-2.5 font-mono text-[#66768D]">{resource.metadata.resourceVersion}</td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    )
}

function WorkloadTable({ pods, deployments }: { pods: BackendPod[]; deployments: BackendDeployment[] }) {
    const rows = [
        ...deployments.map((deployment) => ({
            key: `Deployment/${deployment.ref.namespace}/${deployment.ref.name}`,
            ref: deployment.ref,
            phase: `${deployment.readyReplicas}/${deployment.desiredReplicas} Ready`,
            ready: deployment.readyReplicas >= deployment.desiredReplicas,
            conditions: deployment.conditions,
            owner: deployment.simulatorInstance || '—',
        })),
        ...pods.map((pod) => ({
            key: `Pod/${pod.ref.namespace}/${pod.ref.name}`,
            ref: pod.ref,
            phase: pod.phase,
            ready: pod.ready,
            conditions: pod.conditions,
            owner: pod.simulatorInstance || '—',
        })),
    ]
    return (
        <div className="overflow-hidden rounded-xl border border-white/[0.07] bg-[#0A0E15]/90">
            <div className="flex items-center justify-between border-b border-white/[0.06] px-3.5 py-2.5">
                <span className="text-[10px] font-medium text-[#98A7BB]">Deployment / Pod</span>
                <span className="font-mono text-[9px] text-[#546277]">{rows.length}</span>
            </div>
            {rows.length === 0 ? <EmptyRow text="Informer cache 中没有工作负载" /> : (
                <div className="max-h-[330px] overflow-auto">
                    <table className="w-full min-w-[620px] text-left">
                        <thead className="sticky top-0 border-b border-white/[0.055] bg-[#0A0E15] text-[8px] uppercase tracking-[0.12em] text-[#4F5D71]">
                            <tr>
                                <th className="px-3.5 py-2 font-medium">Kind / Name</th>
                                <th className="px-3.5 py-2 font-medium">Ready / Phase</th>
                                <th className="px-3.5 py-2 font-medium">SimulatorInstance</th>
                                <th className="px-3.5 py-2 font-medium">Conditions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-white/[0.045]">
                            {rows.map((row) => (
                                <tr key={row.key} className="text-[10px] text-[#9BA9BC]">
                                    <td className="px-3.5 py-2.5"><RefName ref={row.ref} /></td>
                                    <td className="px-3.5 py-2.5"><StatusPill label={row.phase} ready={row.ready} /></td>
                                    <td className="px-3.5 py-2.5 font-mono text-[9px]">{row.owner}</td>
                                    <td className="max-w-[280px] px-3.5 py-2.5"><ConditionSummary conditions={row.conditions} /></td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    )
}

function InfrastructureTable({
    nodes,
    services,
    leases,
}: {
    nodes: OverviewData['workloads']['nodes']
    services: OverviewData['workloads']['services']
    leases: OverviewData['workloads']['leases']
}) {
    const rows = [
        ...nodes.map((node) => ({
            key: `Node/${node.ref.name}`,
            ref: node.ref,
            status: node.phase,
            ready: node.ready,
            detail: `${node.role} · ${node.schedulable ? 'schedulable' : 'unschedulable'}${node.zone ? ` · ${node.zone}` : ''}`,
        })),
        ...services.map((service) => ({
            key: `Service/${service.ref.namespace}/${service.ref.name}`,
            ref: service.ref,
            status: service.type,
            ready: Boolean(service.clusterIP && service.clusterIP !== 'None'),
            detail: service.clusterIP || 'headless',
        })),
        ...leases.map((lease) => ({
            key: `Lease/${lease.ref.namespace}/${lease.ref.name}`,
            ref: lease.ref,
            status: lease.fresh ? 'Fresh' : 'Stale',
            ready: lease.fresh,
            detail: lease.holderIdentity || 'no holder',
        })),
    ]
    return (
        <div className="overflow-hidden rounded-xl border border-white/[0.07] bg-[#0A0E15]/90">
            <div className="flex items-center justify-between border-b border-white/[0.06] px-3.5 py-2.5">
                <span className="text-[10px] font-medium text-[#98A7BB]">Node / Service / Lease</span>
                <span className="font-mono text-[9px] text-[#546277]">{rows.length}</span>
            </div>
            {rows.length === 0 ? <EmptyRow text="Informer cache 中没有基础设施资源" /> : (
                <div className="max-h-[320px] overflow-auto">
                    <table className="w-full min-w-[620px] text-left">
                        <thead className="sticky top-0 border-b border-white/[0.055] bg-[#0A0E15] text-[8px] uppercase tracking-[0.12em] text-[#4F5D71]">
                            <tr>
                                <th className="px-3.5 py-2 font-medium">Kind / Name</th>
                                <th className="px-3.5 py-2 font-medium">State</th>
                                <th className="px-3.5 py-2 font-medium">Detail</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-white/[0.045]">
                            {rows.map((row) => (
                                <tr key={row.key} className="text-[10px] text-[#9BA9BC]">
                                    <td className="px-3.5 py-2.5"><RefName ref={row.ref} /></td>
                                    <td className="px-3.5 py-2.5"><StatusPill label={row.status} ready={row.ready} /></td>
                                    <td className="max-w-[320px] truncate px-3.5 py-2.5 font-mono text-[9px] text-[#66768D]">{row.detail}</td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    )
}

function EventTable({ events }: { events: BackendEvent[] }) {
    return (
        <div className="mt-3 max-h-[390px] overflow-auto rounded-xl border border-white/[0.07] bg-[#0A0E15]/90">
            {events.length === 0 ? <EmptyRow text="Kubernetes API 当前没有 Event" /> : (
                <div className="divide-y divide-white/[0.05]">
                    {events.map((event) => (
                        <div key={event.id} className="grid gap-2 p-3.5 sm:grid-cols-[100px_120px_minmax(0,1fr)]">
                            <div className="font-mono text-[9px] text-[#65748A]">{formatTime(event.eventTime)}</div>
                            <div>
                                <StatusPill label={event.reason || event.type} ready={event.type !== 'Warning'} />
                                <div className="mt-1.5 truncate text-[8px] text-[#536177]">{event.regarding.kind}/{event.regarding.name}</div>
                            </div>
                            <p className="text-[10px] leading-4 text-[#9AA8BA]">{event.message || 'No message'}</p>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}

function ProviderTable({ providers, serverTime }: { providers: Record<string, ProviderState>; serverTime: string }) {
    const entries = Object.entries(providers)
    return (
        <div className="mt-3 overflow-hidden rounded-xl border border-white/[0.07] bg-[#0A0E15]/90">
            {entries.length === 0 ? <EmptyRow text="没有 Provider 状态" /> : entries.map(([name, provider]) => (
                <div key={name} className="flex items-start justify-between gap-4 border-b border-white/[0.05] p-3.5 last:border-b-0">
                    <div className="min-w-0">
                        <div className="text-[10px] font-medium capitalize text-[#CAD5E3]">{name}</div>
                        <div className="mt-1 truncate text-[8px] text-[#536178]">{provider.error || provider.storage || 'current state provider'}</div>
                    </div>
                    <div className="shrink-0 text-right">
                        <StatusPill label={provider.state} ready={provider.state === 'ready'} />
                        <div className="mt-1.5 font-mono text-[8px] text-[#506078]">{relativeAge(provider.observedAt, serverTime)}</div>
                    </div>
                </div>
            ))}
        </div>
    )
}

function Sparkline({ points }: { points: MetricPoint[] }) {
    const finite = points.map((point) => point.value).filter(Number.isFinite)
    if (finite.length < 2) {
        return <div className="mt-3 h-8 border-t border-dashed border-white/[0.06] pt-2 text-[8px] text-[#455267]">No samples</div>
    }
    const min = Math.min(...finite)
    const max = Math.max(...finite)
    const range = Math.max(max - min, 1e-9)
    const coordinates = finite.map((value, index) => {
        const x = (index / (finite.length - 1)) * 100
        const y = 29 - ((value - min) / range) * 24
        return `${x},${y}`
    }).join(' ')
    return (
        <svg viewBox="0 0 100 34" preserveAspectRatio="none" className="mt-2 h-9 w-full overflow-visible" aria-hidden="true">
            <path d="M0 31 H100" className="stroke-white/[0.06]" strokeWidth="0.7" />
            <polyline points={coordinates} fill="none" className="stroke-[#689EF7]" strokeWidth="1.4" vectorEffect="non-scaling-stroke" />
        </svg>
    )
}

function MetricTrendChart({
    label,
    metric,
    aggregation,
}: {
    label: string
    metric?: MetricResult
    aggregation: 'sum' | 'average'
}) {
    const points = metric ? aggregateMetricPoints(metric, aggregation) : []
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
                            { offset: 0, color: 'rgba(91,140,255,0.16)' },
                            { offset: 1, color: 'rgba(91,140,255,0)' },
                        ],
                    },
                },
                emphasis: { disabled: true },
            },
        ],
    }
    return (
        <div className="overflow-hidden rounded-xl border border-white/[0.07] bg-[#0A0E15]/90 p-3.5">
            <div className="flex items-center justify-between">
                <span className="text-[10px] font-medium text-[#9AA8BC]">{label}</span>
                <span className="text-[8px] text-[#536177]">{metric?.unit ?? '—'}</span>
            </div>
            <div className="mt-2 h-[110px]">
                <ReactECharts option={option} notMerge style={{ height: '100%', width: '100%' }} />
            </div>
        </div>
    )
}

function isResourceReady(resource: BackendResource) {
    const ready = resource.conditions.find((condition) => condition.type === 'Ready')
    if (ready) return ready.status === 'True'
    const phase = resource.status.phase
    return phase === 'Ready' || phase === 'Running' || phase === 'Active'
}

function ConditionSummary({ conditions }: { conditions: KubernetesCondition[] }) {
    if (conditions.length === 0) return <span className="text-[#4F5D71]">No conditions</span>
    return (
        <div className="flex flex-wrap gap-1">
            {conditions.slice(0, 3).map((condition) => (
                <span
                    key={`${condition.type}/${condition.status}`}
                    title={[condition.reason, condition.message].filter(Boolean).join(': ')}
                    className={`rounded border px-1.5 py-0.5 text-[8px] ${
                        condition.status === 'True'
                            ? 'border-emerald-400/15 bg-emerald-400/[0.055] text-emerald-300'
                            : 'border-amber-400/15 bg-amber-400/[0.055] text-amber-200'
                    }`}
                >
                    {condition.type}:{condition.status}
                </span>
            ))}
        </div>
    )
}

function StatusPill({ label, ready }: { label: string; ready: boolean }) {
    return (
        <span className={`inline-flex max-w-[180px] items-center gap-1 rounded-full border px-2 py-0.5 text-[8px] font-medium ${
            ready
                ? 'border-emerald-400/15 bg-emerald-400/[0.055] text-emerald-300'
                : 'border-amber-400/15 bg-amber-400/[0.055] text-amber-200'
        }`}>
            <span className={`h-1 w-1 shrink-0 rounded-full ${ready ? 'bg-emerald-300' : 'bg-amber-300'}`} />
            <span className="truncate">{label}</span>
        </span>
    )
}

function ProviderDot({ ready }: { ready: boolean }) {
    return <span className={`h-1.5 w-1.5 rounded-full ${ready ? 'bg-emerald-300 shadow-[0_0_7px_rgba(110,231,183,.55)]' : 'bg-[#465267]'}`} />
}

function RefName({ ref }: { ref: ResourceRef }) {
    return (
        <div>
            <div className="max-w-[260px] truncate text-[#D7E1ED]">{ref.name}</div>
            <div className="mt-0.5 text-[8px] text-[#536178]">{ref.kind} · {ref.namespace || 'cluster'}</div>
        </div>
    )
}

function SectionTitle({ icon: Icon, title, subtitle }: { icon: typeof Activity; title: string; subtitle: string }) {
    return (
        <div className="flex items-end justify-between gap-4">
            <div className="flex items-center gap-2">
                <Icon className="h-3.5 w-3.5 text-[#73A7FA]" />
                <h2 className="text-[11px] font-semibold text-[#CFDAE8]">{title}</h2>
            </div>
            <span className="text-[8px] text-[#536177]">{subtitle}</span>
        </div>
    )
}

function SourceBadge({ historical }: { historical: boolean }) {
    return (
        <span className={`rounded-full border px-2 py-0.5 text-[8px] normal-case tracking-normal ${
            historical
                ? 'border-[#9EAEFF]/20 bg-[#9EAEFF]/[0.07] text-[#AEB9FF]'
                : 'border-emerald-400/15 bg-emerald-400/[0.055] text-emerald-300'
        }`}>
            {historical ? 'PostgreSQL Snapshot' : 'Informer Live Cache'}
        </span>
    )
}

function Notice({ text, tone }: { text: string; tone: 'warning' }) {
    return (
        <div data-tone={tone} className="flex items-start gap-2 rounded-lg border border-amber-300/10 bg-amber-300/[0.035] px-3 py-2 text-[9px] leading-4 text-amber-100/75">
            <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0 text-amber-300/70" />
            {text}
        </div>
    )
}

function EmptyRow({ text }: { text: string }) {
    return <div className="px-4 py-8 text-center text-[9px] text-[#536177]">{text}</div>
}

function LoadingState() {
    return (
        <div className="flex min-h-[420px] items-center justify-center gap-2 text-[10px] text-[#66758A]">
            <RefreshCw className="h-4 w-4 animate-spin text-[#6EA3F8]" />
            正在聚合 Kubernetes、Prometheus 与 Jaeger 数据…
        </div>
    )
}

function ErrorState({ message, retry }: { message: string; retry: () => void }) {
    return (
        <div className="mt-6 rounded-xl border border-red-400/15 bg-red-400/[0.035] p-5">
            <div className="flex items-start gap-3">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-red-300" />
                <div>
                    <div className="text-[11px] font-medium text-red-100">Overview API 请求失败</div>
                    <p className="mt-1 text-[10px] leading-5 text-red-100/60">{message}</p>
                    <Button type="button" variant="outline" onClick={retry} className="mt-3 h-7 border-red-300/15 bg-transparent px-3 text-[9px] text-red-100 hover:bg-red-300/[0.07]">
                        重试
                    </Button>
                </div>
            </div>
        </div>
    )
}
