import { useMemo, useState } from 'react'
import {
    BrainCircuit,
    CheckCircle2,
    CircleDashed,
    Clock3,
    Loader2,
    Sparkles,
    XCircle,
} from 'lucide-react'
import { useAIOpsAnalysisBySegment, useAIOpsAnalyses } from '@/api/queries/aiopsQueries'
import { CommandInput } from '@/components/features/observatory/CommandInput'
import { cn } from '@/lib/utils'
import type {
    AIOpsAnalysis,
    AIOpsAnalysisStatus,
    AIOpsClassification,
    AIOpsEntitySummary,
    AIOpsScores,
} from '@/types/aiops.types'

const STATUS_META: Record<AIOpsAnalysisStatus, { label: string; dot: string; text: string }> = {
    pending: { label: '排队中', dot: 'bg-[#5A6778]', text: 'text-[#8C99AC]' },
    running: { label: '实体总结中', dot: 'bg-[#5B8CFF]', text: 'text-[#9EB2FF]' },
    aggregating: { label: '汇总打分中', dot: 'bg-[#C084FC]', text: 'text-[#C4A7F2]' },
    completed: { label: '已完成', dot: 'bg-emerald-400', text: 'text-emerald-300' },
    failed: { label: '失败', dot: 'bg-red-400', text: 'text-red-300' },
}

const CLASSIFICATION_META: Record<AIOpsClassification, { label: string; dot: string; text: string }> = {
    healthy: { label: '优质', dot: 'bg-emerald-400', text: 'text-emerald-300' },
    suspect: { label: '可疑', dot: 'bg-amber-300', text: 'text-amber-200' },
    problem: { label: '问题', dot: 'bg-red-400', text: 'text-red-300' },
}

type ScoreDimKey = 'goal' | 'stability' | 'efficiency' | 'anomaly'

const SCORE_DIMS: Array<{ key: ScoreDimKey; label: string }> = [
    { key: 'goal', label: '目标达成' },
    { key: 'stability', label: '稳定性' },
    { key: 'efficiency', label: '效率' },
    { key: 'anomaly', label: '异常' },
]

function scoreColor(score: number): string {
    if (score >= 80) return 'bg-emerald-400'
    if (score >= 60) return 'bg-amber-300'
    return 'bg-red-400'
}

function formatTime(value: string): string {
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return value
    return new Intl.DateTimeFormat('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
    }).format(date)
}

function shortSegmentId(segmentId: string): string {
    return segmentId.length > 18 ? `${segmentId.slice(0, 8)}…${segmentId.slice(-6)}` : segmentId
}

function StatusBadge({ status }: { status: AIOpsAnalysisStatus }) {
    const meta = STATUS_META[status]
    return (
        <span className={cn('inline-flex items-center gap-1.5 text-[11px] font-medium', meta.text)}>
            <span className={cn('h-1.5 w-1.5 rounded-full', meta.dot)} />
            {meta.label}
        </span>
    )
}

function EntityRow({ entity }: { entity: AIOpsEntitySummary }) {
    const meta = CLASSIFICATION_META[entity.classification]
    return (
        <div className="rounded-lg border border-white/[0.05] bg-white/[0.02] px-3 py-2">
            <div className="flex flex-wrap items-center gap-2">
                <span className={cn('h-1.5 w-1.5 shrink-0 rounded-full', meta.dot)} />
                <span className="font-mono text-[12px] text-[#C6D0DE]">
                    {entity.entityKind} / {entity.entityName}
                </span>
                <span className={cn('rounded-full border border-white/[0.06] px-2 py-0.5 text-[10px]', meta.text)}>
                    {meta.label}
                </span>
                {entity.issueFlag && (
                    <span className="rounded-full border border-red-400/25 bg-red-400/[0.08] px-2 py-0.5 text-[10px] text-red-300">
                        问题标记
                    </span>
                )}
            </div>
            {entity.phenomenon && (
                <p className="mt-1.5 text-[12px] leading-5 text-[#8C99AC]">
                    <span className="text-[#5A6778]">现象：</span>{entity.phenomenon}
                </p>
            )}
            {entity.conclusion && (
                <p className="mt-0.5 text-[12px] leading-5 text-[#B9C5D4]">
                    <span className="text-[#5A6778]">结论：</span>{entity.conclusion}
                </p>
            )}
        </div>
    )
}

function ScoreBar({ label, value }: { label: string; value: number }) {
    return (
        <div>
            <div className="flex items-center justify-between text-[11px]">
                <span className="text-[#8C99AC]">{label}</span>
                <span className="font-mono text-[#C6D0DE]">{value}</span>
            </div>
            <div className="mt-1 h-1 overflow-hidden rounded-full bg-white/[0.06]">
                <div
                    className={cn('h-full rounded-full transition-all', scoreColor(value))}
                    style={{ width: `${Math.min(100, Math.max(0, value))}%` }}
                />
            </div>
        </div>
    )
}

function ScoreDetail({ scores }: { scores: AIOpsScores }) {
    return (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {SCORE_DIMS.map(({ key, label }) => (
                <ScoreBar key={key} label={label} value={scores[key]} />
            ))}
        </div>
    )
}

function AnalysisCard({ analysis, selected, onSelect }: {
    analysis: AIOpsAnalysis
    selected: boolean
    onSelect: () => void
}) {
    const progress = analysis.l1Total > 0 ? analysis.l1Done / analysis.l1Total : 0
    return (
        <button
            type="button"
            onClick={onSelect}
            className={cn(
                'w-full rounded-lg border px-3 py-2.5 text-left transition-colors',
                selected
                    ? 'border-[#5B8CFF]/40 bg-[#5B8CFF]/[0.08]'
                    : 'border-white/[0.05] bg-white/[0.02] hover:border-white/[0.12]',
            )}
        >
            <div className="flex items-center justify-between gap-2">
                <span className="font-mono text-[12px] text-[#C6D0DE]">
                    {shortSegmentId(analysis.segmentId)}
                </span>
                <StatusBadge status={analysis.status} />
            </div>
            <div className="mt-1.5 flex items-center gap-2 text-[11px] text-[#5A6778]">
                <Clock3 className="h-3 w-3" />
                {formatTime(analysis.createdAt)}
                {analysis.l1Total > 0 && (
                    <span className="ml-auto font-mono text-[#8C99AC]">
                        L1 {analysis.l1Done}/{analysis.l1Total}
                    </span>
                )}
            </div>
            {analysis.l1Total > 0 && (
                <div className="mt-1.5 h-0.5 overflow-hidden rounded-full bg-white/[0.06]">
                    <div
                        className={cn(
                            'h-full rounded-full transition-all',
                            analysis.status === 'failed' ? 'bg-red-400' : 'bg-[#5B8CFF]',
                        )}
                        style={{ width: `${Math.round(progress * 100)}%` }}
                    />
                </div>
            )}
        </button>
    )
}

export function AiInsightPanel() {
    const [selectedSegmentId, setSelectedSegmentId] = useState<string | null>(null)
    const [statusFilter, setStatusFilter] = useState<AIOpsAnalysisStatus | null>(null)
    const analysesQuery = useAIOpsAnalyses(statusFilter ?? undefined)
    const detailQuery = useAIOpsAnalysisBySegment(selectedSegmentId)

    const analyses = analysesQuery.data?.data ?? []
    const selected = useMemo(
        () => detailQuery.data?.data.analysis ?? null,
        [detailQuery.data],
    )
    const entities = detailQuery.data?.data.entities ?? []

    const disabled = analysesQuery.error instanceof Error &&
        analysesQuery.error.message.includes('未启用')

    if (disabled) {
        return (
            <div className="rounded-xl border border-white/[0.06] bg-[#0A0E15]/60 px-4 py-8 text-center">
                <BrainCircuit className="mx-auto h-6 w-6 text-[#5A6778]" />
                <p className="mt-3 text-[13px] text-[#8C99AC]">AIOps 分析未启用</p>
                <p className="mt-1 text-[12px] leading-5 text-[#5A6778]">
                    后端需配置 AIOPS_ENABLED=true 且提供 OpenAI API Key，切面完成/失败后自动产出 L1 实体总结与 L2 打分。
                </p>
            </div>
        )
    }

    return (
        <div className="space-y-4">
            <CommandInput />
            <div className="grid grid-cols-1 gap-4 lg:grid-cols-[300px_minmax(0,1fr)]">
            <div className="space-y-2">
                <div className="flex items-center justify-between gap-2 px-1">
                    <span className="text-[11px] font-medium text-[#5A6778]">最近分析（15s 轮询）</span>
                    <div className="flex items-center gap-1">
                        {(
                            [
                                [null, '全部'],
                                ['pending', '排队'],
                                ['running', '分析中'],
                                ['completed', '完成'],
                                ['failed', '失败'],
                            ] as Array<[AIOpsAnalysisStatus | null, string]>
                        ).map(([value, label]) => (
                            <button
                                key={label}
                                type="button"
                                onClick={() => setStatusFilter(value)}
                                className={cn(
                                    'rounded-md px-1.5 py-0.5 text-[10px] transition-colors',
                                    statusFilter === value
                                        ? 'bg-[#5B8CFF]/20 text-[#9EB2FF]'
                                        : 'text-[#5A6778] hover:text-[#8C99AC]',
                                )}
                            >
                                {label}
                            </button>
                        ))}
                    </div>
                    {analysesQuery.isFetching && (
                        <Loader2 className="h-3 w-3 animate-spin text-[#5B8CFF]" />
                    )}
                </div>
                {analyses.length === 0 && !analysesQuery.isLoading && (
                    <div className="rounded-lg border border-dashed border-white/[0.08] px-3 py-6 text-center">
                        <CircleDashed className="mx-auto h-5 w-5 text-[#4C5868]" />
                        <p className="mt-2 text-[12px] text-[#5A6778]">暂无分析记录</p>
                        <p className="mt-0.5 text-[11px] text-[#4C5868]">
                            切面完成或失败后会自动入队分析
                        </p>
                    </div>
                )}
                {analyses.map((analysis) => (
                    <AnalysisCard
                        key={analysis.analysisId}
                        analysis={analysis}
                        selected={analysis.segmentId === selectedSegmentId}
                        onSelect={() => setSelectedSegmentId(analysis.segmentId)}
                    />
                ))}
            </div>

            <div className="min-w-0 rounded-xl border border-white/[0.06] bg-[#0A0E15]/60 p-4">
                {!selected && (
                    <div className="flex h-full min-h-[180px] flex-col items-center justify-center text-center">
                        <Sparkles className="h-6 w-6 text-[#4C5868]" />
                        <p className="mt-3 text-[13px] text-[#8C99AC]">选择左侧分析查看 AI 洞察</p>
                        <p className="mt-1 text-[12px] text-[#5A6778]">分数、理由与实体级总结</p>
                    </div>
                )}
                {selected && (
                    <div className="space-y-5">
                        <div className="flex flex-wrap items-center gap-3">
                            <span className="font-mono text-[13px] text-[#E8EEF7]">
                                {shortSegmentId(selected.segmentId)}
                            </span>
                            <StatusBadge status={selected.status} />
                            {selected.status === 'completed' && selected.scores && (
                                <span className="ml-auto flex items-center gap-2">
                                    <span className="text-[12px] text-[#5A6778]">总分</span>
                                    <span className={cn(
                                        'font-mono text-xl font-semibold',
                                        selected.scores.overall >= 80
                                            ? 'text-emerald-300'
                                            : selected.scores.overall >= 60
                                                ? 'text-amber-200'
                                                : 'text-red-300',
                                    )}>
                                        {selected.scores.overall}
                                    </span>
                                </span>
                            )}
                        </div>

                        {selected.status === 'failed' && (
                            <div className="flex items-start gap-2 rounded-lg border border-red-400/20 bg-red-400/[0.06] px-3 py-2">
                                <XCircle className="mt-0.5 h-4 w-4 shrink-0 text-red-300" />
                                <p className="text-[12px] leading-5 text-red-200/90">
                                    {selected.error || '分析失败，未产出总结与分数。'}
                                </p>
                            </div>
                        )}

                        {selected.status !== 'failed' && (
                            <>
                                {selected.scores ? (
                                    <div className="space-y-3">
                                        <ScoreDetail scores={selected.scores} />
                                        <div className="flex items-start gap-2 rounded-lg border border-white/[0.05] bg-white/[0.02] px-3 py-2.5">
                                            <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-300/90" />
                                            <div>
                                                <p className="text-[12px] font-medium text-[#C6D0DE]">
                                                    {selected.scores.verdict}
                                                </p>
                                                {selected.scores.reason && (
                                                    <p className="mt-1 text-[12px] leading-5 text-[#8C99AC]">
                                                        {selected.scores.reason}
                                                    </p>
                                                )}
                                            </div>
                                        </div>
                                    </div>
                                ) : (
                                    <div className="flex items-center gap-2 text-[12px] text-[#8C99AC]">
                                        <Loader2 className="h-3.5 w-3.5 animate-spin text-[#5B8CFF]" />
                                        正在分析：L1 {selected.l1Done}/{selected.l1Total} 实体
                                        {selected.status === 'aggregating' && ' → 汇总打分中'}
                                    </div>
                                )}

                                <div>
                                    <h3 className="mb-2 flex items-center gap-1.5 text-[12px] font-semibold text-[#C6D0DE]">
                                        <BrainCircuit className="h-3.5 w-3.5 text-[#7CAEFF]" />
                                        实体级总结（{entities.length}）
                                    </h3>
                                    {entities.length === 0 && (
                                        <p className="text-[12px] text-[#5A6778]">暂无实体总结。</p>
                                    )}
                                    <div className="space-y-2">
                                        {entities.map((entity) => (
                                            <EntityRow key={entity.summaryId} entity={entity} />
                                        ))}
                                    </div>
                                </div>
                            </>
                        )}
                    </div>
                )}
                </div>
            </div>
        </div>
    )
}
