import { useState } from 'react'
import {
    CheckCircle2,
    CornerDownLeft,
    Loader2,
    Send,
    Sparkles,
    XCircle,
} from 'lucide-react'
import {
    useConfirmAIOpsCommand,
    useCreateAIOpsCommand,
} from '@/api/queries/aiopsQueries'
import { cn } from '@/lib/utils'
import type { AIOpsCommand, AIOpsCommandStatus } from '@/types/aiops.types'

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
    traffic?: { qps?: number }
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

/**
 * M2 一句话起实验（#94）：输入 → LLM 解析预览 → 用户确认 → 执行。
 * 确认前不产生任何写操作；执行步骤由后端 aiops_commands.steps 返回。
 */
export function CommandInput() {
    const [rawInput, setRawInput] = useState('')
    const [command, setCommand] = useState<AIOpsCommand | null>(null)
    const [error, setError] = useState<string | null>(null)
    const parseMutation = useCreateAIOpsCommand()
    const confirmMutation = useConfirmAIOpsCommand()

    const intent = command ? (command.parsed as ParsedIntent | null) : null
    const statusMeta = command ? STATUS_META[command.status] : null

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

    const selectionCount = intent?.templateSelection
        ? Object.values(intent.templateSelection).reduce((total, ids) => total + (ids?.length ?? 0), 0)
        : 0

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

            <div className="mt-3 flex items-center gap-2">
                <input
                    value={rawInput}
                    onChange={(event) => setRawInput(event.target.value)}
                    onKeyDown={(event) => {
                        if (event.key === 'Enter' && !parseMutation.isPending) handleParse()
                    }}
                    placeholder="例如：美国时间 9 点开始，持续 2 小时，突发流量高峰"
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
