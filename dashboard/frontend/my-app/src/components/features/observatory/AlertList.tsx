import { useQuery } from '@tanstack/react-query'
import {
    AlertTriangle,
    BellOff,
    CheckCheck,
    Siren,
} from 'lucide-react'
import { apiRequest } from '@/api/client'
import { cn } from '@/lib/utils'
import type {
    AIOpsAlertSeverity,
    AIOpsAlertsEnvelope,
} from '@/types/aiops.types'

const SEVERITY_META: Record<AIOpsAlertSeverity, { label: string; text: string; border: string }> = {
    info: { label: '提示', text: 'text-[#9EB2FF]', border: 'border-[#5B8CFF]/25' },
    warning: { label: '警戒', text: 'text-amber-200', border: 'border-amber-300/25' },
    critical: { label: '严重', text: 'text-red-300', border: 'border-red-400/25' },
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

/**
 * 警戒列表（契约先行）：M3 后端未就绪时真实模式返回 404，
 * 显示"未接入"空态；dev:mock 由 fixtures 提供契约演示数据。
 */
export function AlertList() {
    const query = useQuery({
        queryKey: ['aiops', 'alerts'],
        queryFn: () => apiRequest<AIOpsAlertsEnvelope>('/aiops/alerts'),
        refetchInterval: 30_000,
        staleTime: 20_000,
        retry: 0,
    })

    const alerts = query.data?.data ?? []
    const unavailable = Boolean(query.error)

    return (
        <div className="rounded-xl border border-white/[0.06] bg-[#0A0E15]/60 p-4">
            <div className="flex items-center justify-between">
                <h3 className="flex items-center gap-1.5 text-[12px] font-semibold text-[#C6D0DE]">
                    <Siren className="h-3.5 w-3.5 text-[#F0A33B]" />
                    警戒
                    {alerts.length > 0 && (
                        <span className="rounded-full bg-[#F0A33B]/15 px-2 py-0.5 font-mono text-[10px] text-amber-200">
                            {alerts.length}
                        </span>
                    )}
                </h3>
                {query.isFetching && alerts.length > 0 && (
                    <span className="text-[10px] text-[#4C5868]">刷新中…</span>
                )}
            </div>

            {unavailable && (
                <div className="mt-4 flex items-start gap-2 rounded-lg border border-dashed border-white/[0.08] px-3 py-4">
                    <BellOff className="mt-0.5 h-4 w-4 shrink-0 text-[#4C5868]" />
                    <div>
                        <p className="text-[12px] text-[#5A6778]">警戒能力未接入</p>
                        <p className="mt-0.5 text-[11px] leading-4 text-[#4C5868]">
                            后端 M3（时间聚合与警戒）未启用；dev:mock 模式可查看契约演示数据。
                        </p>
                    </div>
                </div>
            )}

            {!unavailable && alerts.length === 0 && (
                <div className="mt-4 rounded-lg border border-dashed border-white/[0.08] px-3 py-4 text-center">
                    <CheckCheck className="mx-auto h-4 w-4 text-emerald-300/80" />
                    <p className="mt-2 text-[12px] text-[#5A6778]">暂无警戒</p>
                </div>
            )}

            {alerts.length > 0 && (
                <div className="mt-3 space-y-2">
                    {alerts.map((alert) => {
                        const meta = SEVERITY_META[alert.severity]
                        return (
                            <div
                                key={alert.alertId}
                                className={cn('rounded-lg border bg-white/[0.02] px-3 py-2.5', meta.border)}
                            >
                                <div className="flex items-center gap-2">
                                    <AlertTriangle className={cn('h-3.5 w-3.5 shrink-0', meta.text)} />
                                    <span className={cn('text-[11px] font-medium', meta.text)}>
                                        {meta.label}
                                    </span>
                                    <span className="ml-auto font-mono text-[10px] text-[#5A6778]">
                                        {formatTime(alert.triggeredAt)}
                                    </span>
                                </div>
                                <p className="mt-1.5 text-[12px] leading-5 text-[#C6D0DE]">{alert.rule}</p>
                                {alert.interpretation != null && (
                                    <p className="mt-0.5 text-[11px] leading-4 text-[#8C99AC]">
                                        {String(alert.interpretation)}
                                    </p>
                                )}
                            </div>
                        )
                    })}
                </div>
            )}
        </div>
    )
}
