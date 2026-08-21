import { useEffect, useState } from 'react'
import {
    CheckCircle2,
    CornerDownLeft,
    Loader2,
    Send,
    Sparkles,
    Square,
    XCircle,
} from 'lucide-react'
import {
    useAIOpsLimits,
    useAIOpsQuota,
    useConfirmAIOpsCommand,
    useCreateAIOpsCommand,
    useStopAIOpsCommand,
} from '@/api/queries/aiopsQueries'
import { fetchAIOpsCommand } from '@/api/endpoints/aiopsApi'
import { cn } from '@/lib/utils'
import type {
    AIOpsAppliedValue,
    AIOpsCommand,
    AIOpsCommandStatus,
    AIOpsTrafficPoint,
    AIOpsTrafficApplied,
} from '@/types/aiops.types'

/** 解析结果的意图形状（与后端 internal/aiops/command.go 对齐）。 */
interface ParsedIntent {
    sceneTimeAnchor?: string
    durationMinutes?: number
    sceneType?: string
    targetTenant?: string
    templateSelection?: {
        modelIds?: string[]
        nodeNames?: string[]
        tenantIds?: string[]
        orchestratorIds?: string[]
        trafficIds?: string[]
    }
    traffic?: { qps?: number; shape?: string; peakQps?: number; periodMinutes?: number }
    rate?: number
}

const STATUS_META: Record<AIOpsCommandStatus, { label: string; text: string }> = {
    parsed: { label: '已解析', text: 'text-[#9EB2FF]' },
    confirmed: { label: '已确认', text: 'text-[#9EB2FF]' },
    gate: { label: '门禁校验', text: 'text-amber-200' },
    executing: { label: '执行中', text: 'text-amber-200' },
    verified: { label: '校验中', text: 'text-amber-200' },
    done: { label: '已完成', text: 'text-emerald-300' },
    rejected: { label: '已拒绝', text: 'text-red-300' },
    failed: { label: '失败', text: 'text-red-300' },
    stopped: { label: '已停止', text: 'text-orange-200' },
}

const FIELD_LABELS: Record<string, string> = {
    peakQps: '峰值 QPS',
    rate: '倍速',
    durationMinutes: '模拟时长',
    periodMinutes: '潮汐周期',
}

const REASON_LABELS: Record<string, string> = {
    'clamped-to-max': '超上限，已钳制',
    defaulted: '未指定，用默认',
    ok: '生效',
}

function shortId(value: string): string {
    return value.length > 18 ? `${value.slice(0, 8)}…${value.slice(-6)}` : value
}

function StepRow({ step }: { step: { step: string; status: string; detail?: string } }) {
    const ok = step.status === 'done'
    return (
        <div className="flex items-start gap-2">
            {ok ? (
                <CheckCircle2 className="mt-0.5 h-3.5 w-3.5 shrink-0 text-emerald-300" />
            ) : (
                <XCircle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-red-300" />
            )}
            <div className="min-w-0">
                <p className="font-mono text-[11px] text-[#C6D0DE]">{step.step}</p>
                {step.detail && (
                    <p className="truncate font-mono text-[10px] text-[#5A6778]">{step.detail}</p>
                )}
            </div>
        </div>
    )
}

/** 波形预览：AI 描绘的生效曲线（#134，SVG 轻量渲染，不引入额外图表库）。 */
function CurvePreview({ curve, peak }: { curve: AIOpsTrafficPoint[]; peak: number }) {
    if (curve.length < 2) return null
    const width = 320
    const height = 48
    const maxX = Math.max(...curve.map((point) => point.x)) || 1
    const maxY = Math.max(peak, ...curve.map((point) => point.y), 1)
    const points = curve
        .map((point) => `${(point.x / maxX) * width},${height - (point.y / maxY) * (height - 6) - 3}`)
        .join(' ')
    const peakY = height - (peak / maxY) * (height - 6) - 3
    return (
        <svg viewBox={`0 0 ${width} ${height}`} className="mt-2 h-12 w-full" preserveAspectRatio="none">
            <line x1="0" y1={peakY} x2={width} y2={peakY} stroke="rgba(91,140,255,.25)" strokeDasharray="3 3" strokeWidth="1" />
            <polyline points={points} fill="none" stroke="#5B8CFF" strokeWidth="1.5" strokeLinejoin="round" />
        </svg>
    )
}

/** 生效值一行：请求值 → 生效值 + 原因（#134：限制可见）。 */
function AppliedRow({ value }: { value: AIOpsAppliedValue }) {
    const label = FIELD_LABELS[value.field] ?? value.field
    const reason = REASON_LABELS[value.reason] ?? value.reason
    if (value.requested == null || value.reason === 'ok') {
        return (
            <span className="text-[11px] text-[#8B98AB]">
                {label} <b className="font-medium text-[#C6D0DE]">{value.effective}</b>
            </span>
        )
    }
    return (
        <span className="text-[11px] text-[#8B98AB]">
            {label} <s className="text-[#5A6778]">{value.requested}</s> →{' '}
            <b className="font-medium text-[#C6D0DE]">{value.effective}</b>
            <em className="not-italic text-amber-200/90">（{reason}）</em>
        </span>
    )
}

function formatWallSeconds(seconds: number): string {
    if (!Number.isFinite(seconds) || seconds <= 0) return '—'
    const minutes = Math.floor(seconds / 60)
    const rest = seconds % 60
    if (minutes >= 60) {
        return `${Math.floor(minutes / 60)}h ${minutes % 60}m`
    }
    return minutes > 0 ? `${minutes}m ${rest}s` : `${rest}s`
}

function formatTokens(value: number): string {
    return value >= 1000 ? `${Math.round(value / 1000)}k` : String(value)
}

/** 从生效曲线插值当前模拟时刻的 QPS（执行中进度展示，#134）。 */
function qpsAt(curve: AIOpsTrafficPoint[], simSeconds: number): number {
    if (curve.length === 0) return 0
    if (simSeconds <= curve[0].x) return curve[0].y
    const last = curve[curve.length - 1]
    if (simSeconds >= last.x) return last.y
    for (let index = 1; index < curve.length; index++) {
        if (curve[index].x >= simSeconds) {
            const prev = curve[index - 1]
            const span = curve[index].x - prev.x
            const ratio = span <= 0 ? 0 : (simSeconds - prev.x) / span
            return Math.round(prev.y + (curve[index].y - prev.y) * ratio)
        }
    }
    return last.y
}

/**
 * M2 一句话起实验（#94/#134）：输入 → LLM 解析预览（含 AI 描绘波形与生效值）→ 用户确认 → 执行。
 * 确认前不产生任何写操作；执行中显示模拟进度/当前 QPS/墙钟剩余，可一键停止。
 */
export function CommandInput() {
    const [rawInput, setRawInput] = useState('')
    const [command, setCommand] = useState<AIOpsCommand | null>(null)
    const [error, setError] = useState<string | null>(null)
    const [nowTick, setNowTick] = useState(0)
    const parseMutation = useCreateAIOpsCommand()
    const confirmMutation = useConfirmAIOpsCommand()
    const stopMutation = useStopAIOpsCommand()
    const limitsQuery = useAIOpsLimits()
    const quotaQuery = useAIOpsQuota()

    const intent = command ? (command.parsed as ParsedIntent | null) : null
    const applied = command?.applied as AIOpsTrafficApplied | undefined
    const statusMeta = command ? STATUS_META[command.status] : null
    const executing = command?.status === 'executing'

    // 执行中：1 秒进度 tick + 2 秒轮询命令状态（结束后自动刷新 done/stopped）。
    useEffect(() => {
        if (!executing) return
        const commandId = command?.commandId
        if (!commandId) return
        const timer = setInterval(() => setNowTick((tick) => tick + 1), 1000)
        const poller = setInterval(() => {
            void fetchAIOpsCommand(commandId)
                .then((envelope) => {
                    setCommand((previous) =>
                        previous && previous.commandId === envelope.data.commandId ? envelope.data : previous,
                    )
                })
                .catch(() => undefined)
        }, 2000)
        return () => {
            clearInterval(timer)
            clearInterval(poller)
        }
    }, [executing, command?.commandId])

    const handleParse = () => {
        const input = rawInput.trim()
        if (!input) return
        setError(null)
        setCommand(null)
        parseMutation.mutate(input, {
            onSuccess: (envelope) => setCommand(envelope.data),
            onError: (cause) => setError(cause instanceof Error ? cause.message : '解析失败'),
        })
    }

    const handleConfirm = () => {
        if (!command) return
        setError(null)
        confirmMutation.mutate(command.commandId, {
            onSuccess: (envelope) => setCommand(envelope.data),
            onError: (cause) => setError(cause instanceof Error ? cause.message : '执行失败'),
        })
    }

    const handleStop = () => {
        if (!command) return
        setError(null)
        stopMutation.mutate(command.commandId, {
            onSuccess: (envelope) => setCommand(envelope.data),
            onError: (cause) => setError(cause instanceof Error ? cause.message : '停止失败'),
        })
    }

    const selectionCount = intent?.templateSelection
        ? Object.values(intent.templateSelection).reduce((total, ids) => total + (ids?.length ?? 0), 0)
        : 0

    // 执行中进度：墙钟已流逝 / 总墙钟；当前 QPS 从生效曲线按模拟时间插值。
    let progress = 0
    let currentQps = 0
    let remainingSeconds = 0
    let totalWallSeconds = 0
    if (executing && command && applied) {
        const startedAt = new Date(command.updatedAt).getTime()
        const elapsedWall = Math.max(0, (Date.now() - startedAt) / 1000)
        totalWallSeconds = applied.wallClockSeconds
        if (totalWallSeconds > 0) {
            progress = Math.min(1, elapsedWall / totalWallSeconds)
            remainingSeconds = Math.max(0, totalWallSeconds - elapsedWall)
        }
        const rate = intent?.rate ?? 1
        const simSeconds = elapsedWall * rate
        currentQps = qpsAt(applied.curve, simSeconds)
    }

    return (
        <div className="rounded-xl border border-white/[0.06] bg-[#0A0E15]/60 p-4">
            <h3 className="flex items-center gap-1.5 text-[12px] font-semibold text-[#C6D0DE]">
                <Sparkles className="h-3.5 w-3.5 text-[#5B8CFF]" />
                一句话起实验
                {command && statusMeta && (
                    <span className={cn('ml-auto text-[11px] font-medium', statusMeta.text)}>
                        {statusMeta.label}
                    </span>
                )}
            </h3>

            {quotaQuery.data?.data.enabled && (
                <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border border-white/[0.05] bg-white/[0.02] px-3 py-1.5 text-[11px] text-[#8B98AB]">
                    <span className="text-[#5A6778]">今日配额：</span>
                    <span>
                        调用 {quotaQuery.data.data.callsUsed}/{quotaQuery.data.data.callsMax}
                    </span>
                    <span>
                        Token {formatTokens(quotaQuery.data.data.tokensUsed)}/
                        {formatTokens(quotaQuery.data.data.tokensMax)}
                    </span>
                </div>
            )}

            {limitsQuery.data && (
                <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border border-white/[0.05] bg-white/[0.02] px-3 py-1.5 text-[11px] text-[#8B98AB]">
                    <span className="text-[#5A6778]">可执行范围：</span>
                    <span>峰值 QPS ≤ {limitsQuery.data.data.maxTrafficQPS}（超限自动钳制并提示）</span>
                    <span>倍速 1-{limitsQuery.data.data.maxSimulationRate}</span>
                    <span>波形 {limitsQuery.data.data.trafficShapes.map((shape) => (
                        shape === 'tidal' ? '潮汐' : shape === 'spike' ? '脉冲' : shape === 'ramp' ? '斜坡' : '平稳'
                    )).join(' / ')}（AI 描绘，无需手画）</span>
                    {limitsQuery.data.data.unlimitedDuration && (
                        <span className="text-[#9EB2FF]">时长不限</span>
                    )}
                    {limitsQuery.data.data.supportsStop && (
                        <span className="text-[#9EB2FF]">随时可停止</span>
                    )}
                </div>
            )}
            <div className="mt-3 flex items-center gap-2">
                <input
                    value={rawInput}
                    onChange={(event) => setRawInput(event.target.value)}
                    onKeyDown={(event) => {
                        if (event.key === 'Enter' && !parseMutation.isPending) handleParse()
                    }}
                    placeholder="例如：给 preset-tenant-001 模拟 2 小时潮汐流量，峰值 50 QPS，倍速 20"
                    className="h-9 flex-1 rounded-lg border border-white/[0.08] bg-white/[0.03] px-3 text-[12px] text-[#C6D0DE] outline-none placeholder:text-[#4C5868] focus:border-[#5B8CFF]/50"
                />
                <button
                    type="button"
                    onClick={handleParse}
                    disabled={parseMutation.isPending || !rawInput.trim()}
                    className="flex h-9 items-center gap-1.5 rounded-lg border border-[#5B8CFF]/30 bg-[#5B8CFF]/[0.12] px-3 text-[12px] font-medium text-[#9EB2FF] transition-colors hover:bg-[#5B8CFF]/[0.2] disabled:cursor-not-allowed disabled:opacity-40"
                >
                    {parseMutation.isPending ? (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                        <CornerDownLeft className="h-3.5 w-3.5" />
                    )}
                    解析
                </button>
            </div>

            {error && (
                <div className="mt-3 flex items-start gap-2 rounded-lg border border-red-400/25 bg-red-400/[0.06] px-3 py-2">
                    <XCircle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-red-300" />
                    <p className="text-[12px] leading-5 text-red-200">{error}</p>
                </div>
            )}

            {intent && (
                <div className="mt-3 rounded-lg border border-white/[0.05] bg-white/[0.02] px-3 py-2.5">
                    <div className="flex flex-wrap gap-x-4 gap-y-1 text-[12px] text-[#C6D0DE]">
                        {intent.sceneType && (
                            <span>
                                <span className="text-[#5A6778]">场景：</span>{intent.sceneType}
                            </span>
                        )}
                        {intent.sceneTimeAnchor && (
                            <span>
                                <span className="text-[#5A6778]">锚点：</span>{intent.sceneTimeAnchor}
                            </span>
                        )}
                        {intent.durationMinutes ? (
                            <span>
                                <span className="text-[#5A6778]">时长：</span>{intent.durationMinutes} 分钟
                            </span>
                        ) : null}
                        {intent.targetTenant && (
                            <span>
                                <span className="text-[#5A6778]">租户：</span>{intent.targetTenant}
                            </span>
                        )}
                        {intent.traffic?.qps != null && (
                            <span>
                                <span className="text-[#5A6778]">流量：</span>{intent.traffic.qps} QPS
                            </span>
                        )}
                        {intent.traffic?.peakQps != null && (
                            <span>
                                <span className="text-[#5A6778]">流量：</span>
                                峰值 {intent.traffic.peakQps} QPS（
                                {intent.traffic.shape === 'tidal' ? '潮汐' : intent.traffic.shape === 'spike' ? '脉冲' : intent.traffic.shape === 'ramp' ? '斜坡' : '波形'}
                                {intent.traffic.periodMinutes ? `，周期 ${intent.traffic.periodMinutes} 分钟` : ''}）
                            </span>
                        )}
                        {intent.rate != null && (
                            <span>
                                <span className="text-[#5A6778]">倍速：</span>{intent.rate}x
                            </span>
                        )}
                        {selectionCount > 0 && (
                            <span>
                                <span className="text-[#5A6778]">模板：</span>{selectionCount} 项选择
                            </span>
                        )}
                    </div>

                    {/* 生效参数 + AI 描绘波形（#134） */}
                    {applied && (
                        <div className="mt-2.5 border-t border-white/[0.05] pt-2.5">
                            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                                <span className="text-[11px] text-[#5A6778]">生效参数：</span>
                                {applied.values.length > 0 ? (
                                    applied.values.map((value) => (
                                        <AppliedRow key={value.field} value={value} />
                                    ))
                                ) : (
                                    <span className="text-[11px] text-[#8B98AB]">全部按请求值执行</span>
                                )}
                                {applied.wallClockSeconds > 0 && (
                                    <span className="text-[11px] text-[#8B98AB]">
                                        墙钟 <b className="font-medium text-[#C6D0DE]">{formatWallSeconds(applied.wallClockSeconds)}</b>
                                        {intent?.rate ? ` @${intent.rate}x` : ''}
                                    </span>
                                )}
                            </div>
                            {applied.curve.length > 1 && (
                                <CurvePreview
                                    curve={applied.curve}
                                    peak={Math.max(...applied.curve.map((point) => point.y))}
                                />
                            )}
                        </div>
                    )}

                    {/* 执行中：进度 / 当前 QPS / 墙钟剩余 / 停止（#134） */}
                    {executing && applied && (
                        <div className="mt-2.5 border-t border-white/[0.05] pt-2.5">
                            <div className="flex items-center justify-between text-[11px] text-[#8B98AB]">
                                <span>
                                    模拟进度 {Math.round(progress * 100)}%（{formatWallSeconds(nowTick)} 已流逝）
                                </span>
                                <span>墙钟剩余 {formatWallSeconds(remainingSeconds)}</span>
                            </div>
                            <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-white/[0.06]">
                                <div
                                    className="h-full rounded-full bg-[#5B8CFF] transition-[width] duration-700"
                                    style={{ width: `${progress * 100}%` }}
                                />
                            </div>
                            <div className="mt-1.5 flex items-center justify-between">
                                <span className="text-[11px] text-[#C6D0DE]">
                                    当前流量 <b className="font-medium text-[#9EB2FF]">{currentQps} QPS</b>
                                </span>
                                <button
                                    type="button"
                                    onClick={handleStop}
                                    disabled={stopMutation.isPending}
                                    className="flex items-center gap-1 rounded-md border border-orange-300/30 bg-orange-300/[0.08] px-2.5 py-1 text-[11px] font-medium text-orange-200 transition-colors hover:bg-orange-300/[0.16] disabled:cursor-not-allowed disabled:opacity-40"
                                >
                                    {stopMutation.isPending ? (
                                        <Loader2 className="h-3 w-3 animate-spin" />
                                    ) : (
                                        <Square className="h-3 w-3" />
                                    )}
                                    停止
                                </button>
                            </div>
                        </div>
                    )}

                    {command?.status === 'done' && command.steps.length > 0 && (
                        <div className="mt-2.5 space-y-1.5 border-t border-white/[0.05] pt-2.5">
                            {command.steps.map((step, index) => (
                                <StepRow key={index} step={step as { step: string; status: string; detail?: string }} />
                            ))}
                        </div>
                    )}
                    {command?.status === 'parsed' && (
                        <button
                            type="button"
                            onClick={handleConfirm}
                            disabled={confirmMutation.isPending}
                            className="mt-2.5 flex w-full items-center justify-center gap-1.5 rounded-lg border border-emerald-300/30 bg-emerald-300/[0.1] px-3 py-2 text-[12px] font-medium text-emerald-200 transition-colors hover:bg-emerald-300/[0.18] disabled:cursor-not-allowed disabled:opacity-40"
                        >
                            {confirmMutation.isPending ? (
                                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                            ) : (
                                <Send className="h-3.5 w-3.5" />
                            )}
                            确认并执行
                        </button>
                    )}
                    {command?.status === 'failed' && command.errorText && (
                        <p className="mt-2 truncate font-mono text-[10px] text-red-300">{command.errorText}</p>
                    )}
                    <p className="mt-2 font-mono text-[10px] text-[#4C5868]">
                        {shortId(command?.commandId ?? '')}
                    </p>
                </div>
            )}
        </div>
    )
}
