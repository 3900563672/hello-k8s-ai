import { useEffect, useMemo, useState } from 'react'
import {
    Activity,
    Boxes,
    Gauge,
    LayoutDashboard,
    RefreshCw,
    Sparkles,
    Spline,
    TimerReset,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useAIOpsAnalyses, useAIOpsAnalysisBySegment } from '@/api/queries/aiopsQueries'
import { useOverview } from '@/api/queries/traceQueries'
import { useReplayTimeContext } from '@/stores/timeSlice'
import { CollapsibleSection } from '@/components/shared/CollapsibleSection'
import { AiInsightPanel } from '@/components/features/observatory/AiInsightPanel'
import { AlertList } from '@/components/features/observatory/AlertList'
import { ClusterBubbleField } from '@/components/features/observatory/ClusterBubbleField'
import { CommandInput } from '@/components/features/observatory/CommandInput'
import { TraceWaterfall } from '@/components/features/observatory/TraceWaterfall'
import { MonitorWall } from '@/components/features/monitor/MonitorWall'
import { SegmentPanel } from '@/components/features/trace/SegmentPanel'
import { ExperimentPanel } from '@/components/features/trace/ExperimentPanel'
import type { AgentVerdict } from '@/types/aiops.types'
import { cn } from '@/lib/utils'

const GRAFANA_BASE = '/grafana'
const GRAFANA_OVERVIEW = `${GRAFANA_BASE}/d/hello-k8s-ai-overview?kiosk`

const SECTIONS = [
    { id: 'topology', label: '集群拓扑', icon: Boxes },
    { id: 'metrics', label: '实时指标', icon: Gauge },
    { id: 'grafana', label: 'Grafana', icon: Activity },
    { id: 'segments', label: '调度与切面', icon: TimerReset },
    { id: 'ai', label: 'AI 洞察', icon: Sparkles },
    { id: 'traces', label: '调用链', icon: Spline },
] as const

function SectionTitle({ index, title, subtitle }: { index: string; title: string; subtitle: string }) {
    return (
        <div className="flex items-center gap-3">
            <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-[#5B8CFF]/15 text-[12px] font-semibold text-[#9EB2FF]">
                {index}
            </span>
            <div>
                <h2 className="text-[15px] font-semibold text-[#E8EEF7]">{title}</h2>
                <p className="mt-0.5 text-[12px] text-[#5A6778]">{subtitle}</p>
            </div>
        </div>
    )
}

function useActiveSection(): string {
    const [active, setActive] = useState<string>(SECTIONS[0].id)
    useEffect(() => {
        const observer = new IntersectionObserver(
            (entries) => {
                for (const entry of entries) {
                    if (entry.isIntersecting) {
                        setActive(entry.target.id)
                    }
                }
            },
            { rootMargin: '-20% 0px -70% 0px' },
        )
        for (const section of SECTIONS) {
            const element = document.getElementById(section.id)
            if (element) observer.observe(element)
        }
        return () => observer.disconnect()
    }, [])
    return active
}

export function ObservatoryPage() {
    const replay = useReplayTimeContext()
    const query = useOverview(replay)
    const overview = query.data?.data
    const active = useActiveSection()

    const scrollTo = (id: string) => {
        document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }

    const stats = useMemo(() => {
        const nodes = overview?.workloads.nodes.length ?? 0
        const pods = overview?.workloads.pods.length ?? 0
        const tenants = overview?.traffic.tenants.length ?? 0
        const traces = overview?.traces.length ?? 0
        const readyPods = overview?.workloads.pods.filter((pod) => pod.ready).length ?? 0
        const readyNodes = overview?.workloads.nodes.filter((node) => node.ready).length ?? 0
        return { nodes, readyNodes, pods, readyPods, tenants, traces }
    }, [overview])

    // AI 洞察 → 气泡外圈：取最新 completed 分析的 L1 实体总结注入 overview。
    const aiopsAnalyses = useAIOpsAnalyses()
    const latestCompletedSegmentId = useMemo(() => {
        const completed = aiopsAnalyses.data?.data.find(
            (analysis) => analysis.status === 'completed',
        )
        return completed?.segmentId ?? null
    }, [aiopsAnalyses.data])
    const aiopsDetail = useAIOpsAnalysisBySegment(latestCompletedSegmentId)
    const verdicts = useMemo(() => {
        const map = new Map<string, AgentVerdict>()
        const detail = aiopsDetail.data?.data
        if (!detail) return map
        const overall = detail.analysis.scores?.overall
        for (const entity of detail.entities) {
            const grade =
                entity.classification === 'healthy'
                    ? 'normal'
                    : entity.classification === 'suspect'
                        ? 'odd'
                        : 'problematic'
            map.set(`${entity.entityKind}:${entity.entityName}`, {
                grade,
                score: overall,
                summary: entity.conclusion,
                updatedAt: entity.createdAt,
            })
        }
        return map
    }, [aiopsDetail.data])
    const overviewWithVerdicts = useMemo(() => {
        if (!overview || verdicts.size === 0) return overview
        return {
            ...overview,
            workloads: {
                ...overview.workloads,
                pods: overview.workloads.pods.map((pod) => {
                    const verdict = verdicts.get(`Pod:${pod.ref.name}`)
                    return verdict ? { ...pod, agentVerdict: verdict } : pod
                }),
                nodes: overview.workloads.nodes.map((node) => {
                    const verdict = verdicts.get(`Node:${node.ref.name}`)
                    return verdict ? { ...node, agentVerdict: verdict } : node
                }),
            },
            traffic: {
                ...overview.traffic,
                tenants: overview.traffic.tenants.map((tenant) => {
                    const verdict = verdicts.get(`Tenant:${tenant.tenant.name}`)
                    return verdict ? { ...tenant, agentVerdict: verdict } : tenant
                }),
            },
        }
    }, [overview, verdicts])

    return (
        <div className="relative h-full overflow-auto bg-[#05070A] text-[#E8EEF7]">
            <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_56%_6%,rgba(91,140,255,.08),transparent_28%)]" />
            <main className="relative mx-auto w-full max-w-[1500px] px-5 py-6 lg:px-8 lg:py-8">
                <header className="flex flex-col gap-4 border-b border-white/[0.07] pb-5 lg:flex-row lg:items-end lg:justify-between">
                    <div>
                        <div className="flex items-center gap-2 text-[12px] font-medium uppercase tracking-[0.16em] text-[#6B788C]">
                            <LayoutDashboard className="h-3.5 w-3.5 text-[#7CAEFF]" />
                            Observatory / 统一状态
                        </div>
                        <h1 className="mt-3 text-2xl font-semibold tracking-[-0.025em] text-[#F0F5FB]">
                            状态总览
                        </h1>
                        <p className="mt-1.5 text-[14px] text-[#657286]">
                            Informer cache、Prometheus、Jaeger 与 PostgreSQL 时间切面的统一视图
                        </p>
                        <div className="mt-3 flex flex-wrap items-center gap-2">
                            <span className="rounded-lg border border-white/[0.07] bg-white/[0.02] px-2.5 py-1 text-[12px] text-[#8C99AC]">
                                节点 <strong className="font-mono text-[#C6D0DE]">{stats.readyNodes}/{stats.nodes}</strong>
                            </span>
                            <span className="rounded-lg border border-white/[0.07] bg-white/[0.02] px-2.5 py-1 text-[12px] text-[#8C99AC]">
                                Pod <strong className="font-mono text-[#C6D0DE]">{stats.readyPods}/{stats.pods}</strong>
                            </span>
                            <span className="rounded-lg border border-white/[0.07] bg-white/[0.02] px-2.5 py-1 text-[12px] text-[#8C99AC]">
                                租户 <strong className="font-mono text-[#C6D0DE]">{stats.tenants}</strong>
                            </span>
                            <span className="rounded-lg border border-white/[0.07] bg-white/[0.02] px-2.5 py-1 text-[12px] text-[#8C99AC]">
                                Trace <strong className="font-mono text-[#C6D0DE]">{stats.traces}</strong>
                            </span>
                        </div>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                        <Button
                            type="button"
                            variant="outline"
                            disabled={query.isFetching}
                            onClick={() => void query.refetch()}
                            className="h-8 gap-2 border-white/[0.08] bg-white/[0.025] px-3 text-[12px] text-[#AAB6C8] hover:bg-white/[0.06] hover:text-white"
                        >
                            <RefreshCw className={`h-3.5 w-3.5 ${query.isFetching ? 'animate-spin' : ''}`} />
                            刷新
                        </Button>

                    </div>
                </header>

                <div className="mt-6 flex gap-6">
                    <aside className="sticky top-6 hidden w-48 shrink-0 self-start xl:block">
                        <nav className="space-y-1 rounded-xl border border-white/[0.06] bg-[#090D14]/80 p-2">
                            {SECTIONS.map((section) => {
                                const isActive = active === section.id
                                return (
                                    <button
                                        key={section.id}
                                        type="button"
                                        onClick={() => scrollTo(section.id)}
                                        className={cn(
                                            'flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-[12px] transition-colors',
                                            isActive
                                                ? 'bg-[#5B8CFF]/[0.12] text-[#C9DDFF]'
                                                : 'text-[#7A8799] hover:bg-white/[0.04] hover:text-[#C6D0DE]',
                                        )}
                                    >
                                        <section.icon className={cn('h-3.5 w-3.5', isActive ? 'text-[#7CAEFF]' : 'text-[#5A6778]')} />
                                        {section.label}
                                    </button>
                                )
                            })}
                        </nav>
                        <p className="mt-3 px-2 text-[11px] leading-4 text-[#4C5868]">
                            各信息域独立分区：拓扑、指标、切面与调用链互不混排。
                        </p>
                    </aside>

                    <div className="min-w-0 flex-1 space-y-8">
                        <section id="topology" className="scroll-mt-6 rounded-2xl border border-white/[0.07] bg-[#090D14]/70 p-4 lg:p-5">
                            <SectionTitle index="01" title="集群拓扑" subtitle="节点 / Pod / 租户气泡场，颜色表示健康度，外圈表示 Agent 分级" />
                            <div className="mt-4">
                                <ClusterBubbleField overview={overviewWithVerdicts} />
                            </div>
                        </section>

                        <section id="metrics" className="scroll-mt-6 rounded-2xl border border-white/[0.07] bg-[#090D14]/70 p-4 lg:p-5">
                            <SectionTitle index="02" title="实时指标" subtitle="Simulator 关键指标唯一展示位（TTFT / QPS / 队列 / Tick 延迟 / 错误率）" />
                            <div className="mt-4">
                                <MonitorWall />
                            </div>
                        </section>

                        <section id="grafana" className="scroll-mt-6 rounded-2xl border border-white/[0.07] bg-[#090D14]/70 p-4 lg:p-5">
                            <SectionTitle index="03" title="Grafana" subtitle="外部可观测面板总览（hello-k8s-ai-overview）" />
                            <div className="mt-4 overflow-hidden rounded-xl border border-white/[0.07]">
                                <iframe
                                    src={GRAFANA_OVERVIEW}
                                    title="Grafana 总览"
                                    className="h-[540px] w-full bg-[#090D14]"
                                />
                            </div>
                        </section>

                        <section id="segments" className="scroll-mt-6 rounded-2xl border border-white/[0.07] bg-[#090D14]/70 p-4 lg:p-5">
                            <SectionTitle index="04" title="调度与切面" subtitle="时间段切面聚合分析、实验生命周期与资源事件" />
                            <div className="mt-4 space-y-3">
                                <CollapsibleSection title="时间段切面分析" subtitle="选择起点与终点快照，聚合区间指标与 Trace">
                                    <SegmentPanel />
                                </CollapsibleSection>
                                <CollapsibleSection title="切面实验" subtitle="实验创建、执行与结果记录">
                                    <ExperimentPanel />
                                </CollapsibleSection>
                            </div>
                        </section>

                        <section id="ai" className="scroll-mt-6 rounded-2xl border border-white/[0.07] bg-[#090D14]/70 p-4 lg:p-5">
                            <SectionTitle index="05" title="AI 洞察" subtitle="切面分层总结与打分、一句话起实验入口与警戒" />
                            <div className="mt-4 space-y-3">
                                <CommandInput />
                                <div className="grid grid-cols-1 gap-3 xl:grid-cols-[minmax(0,1fr)_300px]">
                                    <AiInsightPanel />
                                    <AlertList />
                                </div>
                            </div>
                        </section>

                        <section id="traces" className="scroll-mt-6 rounded-2xl border border-white/[0.07] bg-[#090D14]/70 p-4 lg:p-5">
                            <SectionTitle index="06" title="调用链" subtitle="Trace 瀑布图：父子缩进、耗时条宽、关键路径高亮与 Span 详情" />
                            <div className="mt-4">
                                <TraceWaterfall traces={overview?.traces ?? []} />
                            </div>
                        </section>
                    </div>
                </div>
            </main>
        </div>
    )
}
