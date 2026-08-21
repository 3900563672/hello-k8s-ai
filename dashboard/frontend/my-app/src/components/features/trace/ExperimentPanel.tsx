import { useState } from 'react'
import {
    Activity,
    CheckCircle2,
    Clock3,
    FlaskConical,
    Play,
    RefreshCw,
    Square,
    XCircle,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
    useCompleteExperiment,
    useCreateExperiment,
    useExperimentDetail,
    useExperiments,
    useFailExperiment,
    useStartExperiment,
} from '@/api/queries/experimentQueries'
import type { ExperimentRecord, ExperimentStatus } from '@/types/experiment.types'

const statusLabels: Record<ExperimentStatus, string> = {
    pending: '待开始',
    running: '运行中',
    completed: '已完成',
    failed: '已失败',
}

const eventTypeLabels: Record<string, string> = {
    decision: '决策',
    alert: '告警',
    error: '错误',
    gap: '缺口',
    burst: '突变',
    phase_change: '阶段变化',
}

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

function statusBadgeClass(status: ExperimentStatus) {
    switch (status) {
        case 'running':
            return 'border-emerald-400/25 bg-emerald-400/[0.08] text-emerald-300'
        case 'completed':
            return 'border-[#5B8CFF]/25 bg-[#5B8CFF]/[0.08] text-[#AFCBFF]'
        case 'failed':
            return 'border-red-400/25 bg-red-400/[0.08] text-red-300'
        default:
            return 'border-white/[0.12] bg-white/[0.04] text-[#9AA8BC]'
    }
}

function ExperimentActions({ record, onSelected }: {
    record: ExperimentRecord
    onSelected: (segmentId: string) => void
}) {
    const start = useStartExperiment()
    const complete = useCompleteExperiment()
    const fail = useFailExperiment()
    const [failOpen, setFailOpen] = useState(false)
    const [reason, setReason] = useState('')

    if (record.status === 'pending') {
        return (
            <Button
                type="button"
                variant="outline"
                size="xs"
                disabled={start.isPending}
                onClick={() => start.mutate(record.segmentId)}
                className="gap-1.5 border-[#5B8CFF]/25 bg-[#5B8CFF]/[0.06] px-2.5 text-[13px] text-[#AFCBFF] hover:bg-[#5B8CFF]/[0.12] hover:text-white"
            >
                <Play className="h-3 w-3" />
                开始
            </Button>
        )
    }
    if (record.status === 'running') {
        return (
            <div className="flex items-center gap-1.5">
                <Button
                    type="button"
                    variant="outline"
                    size="xs"
                    disabled={complete.isPending}
                    onClick={() => complete.mutate(record.segmentId)}
                    className="gap-1.5 border-emerald-400/20 bg-emerald-400/[0.06] px-2.5 text-[13px] text-emerald-300 hover:bg-emerald-400/[0.12] hover:text-white"
                >
                    <Square className="h-3 w-3" />
                    完成
                </Button>
                {failOpen ? (
                    <div className="flex items-center gap-1.5">
                        <Input
                            value={reason}
                            onChange={(event) => setReason(event.target.value)}
                            placeholder="失败原因…"
                            className="h-6 w-32 border-[#303C50] bg-[#0D1420] px-2 text-[13px] text-[#C7D3E2] placeholder:text-[#556] focus-visible:border-[#5B8CFF]/40"
                        />
                        <Button
                            type="button"
                            variant="outline"
                            size="xs"
                            disabled={fail.isPending}
                            onClick={() => fail.mutate({ segmentId: record.segmentId, reason })}
                            className="gap-1 border-red-400/20 bg-red-400/[0.06] px-2 text-[13px] text-red-300 hover:bg-red-400/[0.12] hover:text-white"
                        >
                            确认失败
                        </Button>
                    </div>
                ) : (
                    <Button
                        type="button"
                        variant="outline"
                        size="xs"
                        onClick={() => setFailOpen(true)}
                        className="gap-1.5 border-red-400/20 bg-red-400/[0.06] px-2.5 text-[13px] text-red-300 hover:bg-red-400/[0.12] hover:text-white"
                    >
                        <XCircle className="h-3 w-3" />
                        失败
                    </Button>
                )}
            </div>
        )
    }
    return (
        <Button
            type="button"
            variant="outline"
            size="xs"
            onClick={() => onSelected(record.segmentId)}
            className="gap-1.5 border-white/[0.08] bg-white/[0.025] px-2.5 text-[13px] text-[#AAB6C8] hover:bg-white/[0.06] hover:text-white"
        >
            查看
        </Button>
    )
}

function ExperimentDetailView({ segmentId }: { segmentId: string }) {
    const detail = useExperimentDetail(segmentId)
    const experiment = detail.data?.data
    if (detail.isPending) {
        return (
            <div className="flex items-center justify-center gap-2 px-4 py-8 text-[12px] text-[#66758A]">
                <RefreshCw className="h-3.5 w-3.5 animate-spin text-[#6EA3F8]" />
                正在加载实验详情…
            </div>
        )
    }
    if (detail.isError || !experiment) {
        return (
            <div className="px-4 py-5 text-[13px] leading-4 text-red-300">
                实验详情请求失败：{detail.error?.message ?? '未知错误'}
            </div>
        )
    }
    const summary = experiment.segment.summary
    return (
        <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-3 text-[13px] text-[#68768A]">
                <span>
                    时长 <span className="text-[#C7D3E2]">{summary?.durationSeconds ?? 0}s</span>
                </span>
                <span>
                    事件 <span className="text-[#C7D3E2]">{experiment.events.length}</span>
                </span>
                <span>
                    指标分桶 <span className="text-[#C7D3E2]">{summary?.metricBuckets ?? 0}</span>
                </span>
                <span>
                    Trace <span className="text-[#C7D3E2]">{experiment.traces.length}</span>
                </span>
                {experiment.segment.reason && (
                    <span className="text-red-300">原因：{experiment.segment.reason}</span>
                )}
            </div>

            {experiment.events.length > 0 && (
                <div>
                    <div className="mb-1.5 text-[13px] font-medium text-[#9AA8BC]">事件序列</div>
                    <div className="max-h-44 overflow-y-auto rounded-lg border border-white/[0.06]">
                        {experiment.events.map((event) => (
                            <div
                                key={event.eventId}
                                className="flex items-center justify-between gap-3 border-b border-white/[0.04] px-3 py-1.5 last:border-0"
                            >
                                <div className="flex min-w-0 items-center gap-2">
                                    <span className={event.severity === 'warning'
                                        ? 'text-[#E8B36A]'
                                        : 'text-[#7CAEFF]'}>
                                        {event.severity === 'warning' ? '▲' : '●'}
                                    </span>
                                    <span className="w-14 shrink-0 text-[13px] text-[#D8E2EF]">
                                        {eventTypeLabels[event.eventType] ?? event.eventType}
                                    </span>
                                    <span className="truncate text-[13px] text-[#68768A]">{event.entity}</span>
                                </div>
                                <span className="shrink-0 font-mono text-[12px] text-[#506077]">
                                    {formatTime(event.occurredAt)}
                                </span>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {experiment.metrics.length > 0 && (
                <div>
                    <div className="mb-1.5 text-[13px] font-medium text-[#9AA8BC]">指标分桶（1min min/avg/max/p95）</div>
                    <div className="max-h-44 overflow-y-auto rounded-lg border border-white/[0.06]">
                        {experiment.metrics.slice(0, 60).map((bucket, index) => (
                            <div
                                key={`${bucket.metricName}-${bucket.bucketStart}-${index}`}
                                className="flex items-center justify-between gap-3 border-b border-white/[0.04] px-3 py-1.5 last:border-0"
                            >
                                <span className="w-32 shrink-0 truncate text-[13px] text-[#D8E2EF]">{bucket.metricName}</span>
                                <span className="font-mono text-[12px] text-[#506077]">{formatTime(bucket.bucketStart)}</span>
                                <span className="shrink-0 font-mono text-[12px] text-[#AAB6C8]">
                                    {number.format(bucket.min)} / {number.format(bucket.avg)} / {number.format(bucket.max)} / {number.format(bucket.p95)}
                                </span>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {experiment.traces.length > 0 && (
                <div>
                    <div className="mb-1.5 text-[13px] font-medium text-[#9AA8BC]">关联 Trace</div>
                    <div className="max-h-40 overflow-y-auto rounded-lg border border-white/[0.06]">
                        {experiment.traces.map((trace) => (
                            <div
                                key={trace.traceId}
                                className="flex items-center justify-between gap-3 border-b border-white/[0.04] px-3 py-1.5 last:border-0"
                            >
                                <div className="min-w-0">
                                    <div className="truncate text-[13px] text-[#D8E2EF]">
                                        {trace.rootService} · {trace.rootOperation}
                                    </div>
                                    <div className="truncate font-mono text-[12px] text-[#506077]">{trace.traceId}</div>
                                </div>
                                <span className="shrink-0 text-[12px] text-[#68768A]">
                                    {number.format(trace.durationMs)} ms
                                </span>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {experiment.events.length === 0 && experiment.metrics.length === 0 && (
                <div className="px-2 py-4 text-center text-[13px] text-[#536177]">
                    该实验还没有事件与指标——开始运行后由混合采样器持续写入
                </div>
            )}
        </div>
    )
}

export function ExperimentPanel() {
    const experiments = useExperiments()
    const create = useCreateExperiment()
    const [selectedId, setSelectedId] = useState<string | null>(null)
    const [tenant, setTenant] = useState('default')
    const [name, setName] = useState('')
    const records = experiments.data?.data ?? []
    const selected = selectedId ?? records.find((record) => record.status === 'running')?.segmentId ?? null

    const createExperiment = () => {
        const trimmedName = name.trim()
        if (!trimmedName) return
        create.mutate(
            { tenant: tenant.trim() || 'default', name: trimmedName },
            {
                onSuccess: (envelope) => {
                    setSelectedId(envelope.data.segment.segmentId)
                    setName('')
                },
            },
        )
    }

    return (
        <section className="rounded-xl border border-[#5B8CFF]/12 bg-[#0A0E15]/70">
            <div className="flex flex-col gap-3 border-b border-white/[0.06] p-4 lg:flex-row lg:items-center lg:justify-between">
                <div>
                    <div className="flex items-center gap-2 text-[12px] font-medium text-[#9AA8BC]">
                        <FlaskConical className="h-3.5 w-3.5 text-[#7CAEFF]" />
                        实验切面（Experiment）
                        <span className="rounded-full border border-[#9EAEFF]/20 bg-[#9EAEFF]/[0.07] px-2 py-0.5 text-[12px] text-[#AEB9FF]">
                            生命周期 + 混合采样 + 分层存储
                        </span>
                    </div>
                    <p className="mt-1 text-[13px] text-[#657286]">
                        一次调度实验的不可变归档：pending → running → completed / failed；运行中由后台采样器写入事件、指标分桶与 Trace 关联
                    </p>
                </div>
                <div className="flex items-center gap-2">
                    <Input
                        value={tenant}
                        onChange={(event) => setTenant(event.target.value)}
                        placeholder="租户"
                        className="h-8 w-24 border-white/[0.08] bg-[#0D1420] px-2 text-[12px] text-[#C7D3E2] placeholder:text-[#556] focus-visible:border-[#5B8CFF]/40"
                    />
                    <Input
                        value={name}
                        onChange={(event) => setName(event.target.value)}
                        onKeyDown={(event) => {
                            if (event.key === 'Enter') {
                                event.preventDefault()
                                createExperiment()
                            }
                        }}
                        placeholder="实验名称，例如：扩容验证-20x"
                        className="h-8 w-52 border-white/[0.08] bg-[#0D1420] px-2 text-[12px] text-[#C7D3E2] placeholder:text-[#556] focus-visible:border-[#5B8CFF]/40"
                    />
                    <Button
                        type="button"
                        variant="outline"
                        disabled={!name.trim() || create.isPending}
                        onClick={createExperiment}
                        className="h-8 gap-2 border-[#5B8CFF]/25 bg-[#5B8CFF]/[0.06] px-3 text-[12px] text-[#AFCBFF] hover:bg-[#5B8CFF]/[0.12] hover:text-white"
                    >
                        {create.isPending ? (
                            <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                        ) : (
                            <FlaskConical className="h-3.5 w-3.5" />
                        )}
                        创建实验
                    </Button>
                </div>
            </div>

            {create.isError && (
                <div className="px-4 py-2 text-[13px] text-red-300">
                    创建实验失败：{create.error.message}
                </div>
            )}

            <div className="grid gap-4 p-4 lg:grid-cols-2">
                <div>
                    <div className="mb-1.5 text-[13px] font-medium text-[#9AA8BC]">
                        实验列表（自动刷新）
                    </div>
                    <div className="max-h-72 overflow-y-auto rounded-lg border border-white/[0.06]">
                        {experiments.isPending && (
                            <div className="flex items-center justify-center gap-2 px-4 py-6 text-[13px] text-[#66758A]">
                                <RefreshCw className="h-3 w-3 animate-spin text-[#6EA3F8]" />
                                加载中…
                            </div>
                        )}
                        {!experiments.isPending && records.length === 0 && (
                            <div className="px-4 py-6 text-center text-[13px] text-[#536177]">
                                还没有实验——先创建并开始一个实验，混合采样器会持续沉淀数据
                            </div>
                        )}
                        {records.map((record) => (
                            <div
                                key={record.segmentId}
                                className={`flex cursor-pointer items-center justify-between gap-3 border-b border-white/[0.04] px-3 py-2 last:border-0 ${
                                    selected === record.segmentId ? 'bg-[#5B8CFF]/[0.05]' : 'hover:bg-white/[0.02]'
                                }`}
                                onClick={() => setSelectedId(record.segmentId)}
                            >
                                <div className="min-w-0">
                                    <div className="flex items-center gap-2">
                                        <span className={`rounded-full border px-1.5 py-0.5 text-[12px] ${statusBadgeClass(record.status)}`}>
                                            {statusLabels[record.status]}
                                        </span>
                                        <span className="truncate text-[12px] font-medium text-[#D8E2EF]">{record.name}</span>
                                    </div>
                                    <div className="mt-0.5 flex items-center gap-2 text-[12px] text-[#506077]">
                                        <span>{record.tenant}</span>
                                        <Clock3 className="h-2.5 w-2.5" />
                                        <span>{formatTime(record.startedAt ?? record.createdAt)}</span>
                                    </div>
                                </div>
                                <div onClick={(event) => event.stopPropagation()}>
                                    <ExperimentActions record={record} onSelected={setSelectedId} />
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
                <div>
                    <div className="mb-1.5 flex items-center gap-2 text-[13px] font-medium text-[#9AA8BC]">
                        {selected ? '实验详情' : '详情'}
                        {selected && (
                            <span className="flex items-center gap-1 text-[12px] text-[#68768A]">
                                {selected === records.find((record) => record.status === 'running')?.segmentId && (
                                    <span className="flex items-center gap-1 text-emerald-300">
                                        <Activity className="h-2.5 w-2.5" />
                                        采样中
                                    </span>
                                )}
                            </span>
                        )}
                    </div>
                    {selected ? (
                        <ExperimentDetailView segmentId={selected} />
                    ) : (
                        <div className="flex items-center gap-2 rounded-lg border border-white/[0.06] px-4 py-6 text-[13px] text-[#536177]">
                            <CheckCircle2 className="h-3.5 w-3.5 text-[#7CAEFF]" />
                            选择左侧实验查看事件、指标分桶与关联 Trace
                        </div>
                    )}
                </div>
            </div>
        </section>
    )
}