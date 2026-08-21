import { useQuery } from '@tanstack/react-query'
import { CalendarRange, History, Loader2 } from 'lucide-react'
import { apiRequest } from '@/api/client'
import { cn } from '@/lib/utils'
import type {
    AIOpsWindowLevel,
    AIOpsWindowSummary,
    AIOpsWindowsEnvelope,
} from '@/types/aiops.types'

const LEVEL_META: Record<AIOpsWindowLevel, { label: string; text: string }> = {
    L3: { label: '窗口总结', text: 'text-[#9EB2FF]' },
    L4: { label: '日总结', text: 'text-[#C4A7F2]' },
}

function formatRange(window: AIOpsWindowSummary): string {
    const format = (value: string) => {
        const date = new Date(value)
        return Number.isNaN(date.getTime())
            ? value
            : new Intl.DateTimeFormat('zh-CN', {
                  month: '2-digit',
                  day: '2-digit',
                  hour: '2-digit',
                  minute: '2-digit',
              }).format(date)
    }
    return `${format(window.windowStart)} ~ ${format(window.windowEnd)}`
}

/**
 * L3/L4 时间聚合总结入口（契约先行）：M3 后端未就绪时 404 →
 * 显示未接入空态；dev:mock 由 fixtures 提供契约演示数据。
 */
export function WindowSummaryPanel() {
    const query = useQuery({
        queryKey: ['aiops', 'windows'],
        queryFn: () => apiRequest<AIOpsWindowsEnvelope>('/aiops/windows'),
        refetchInterval: 30_000,
        staleTime: 20_000,
        retry: 0,
    })

    const windows = query.data?.data ?? []
    const unavailable = Boolean(query.error)

    return (
        <div className="rounded-xl border border-white/[0.06] bg-[#0A0E15]/60 p-4">
            <div className="flex items-center justify-between">
                <h3 className="flex items-center gap-1.5 text-[12px] font-semibold text-[#C6D0DE]">
                    <CalendarRange className="h-3.5 w-3.5 text-[#9EB2FF]" />
                    窗口与日总结
                </h3>
                {query.isFetching && windows.length > 0 && (
                    <Loader2 className="h-3 w-3 animate-spin text-[#5B8CFF]" />
                )}
            </div>

            {unavailable && (
                <div className="mt-4 flex items-start gap-2 rounded-lg border border-dashed border-white/[0.08] px-3 py-4">
                    <History className="mt-0.5 h-4 w-4 shrink-0 text-[#4C5868]" />
                    <div>
                        <p className="text-[12px] text-[#5A6778]">时间聚合未接入</p>
                        <p className="mt-0.5 text-[11px] leading-4 text-[#4C5868]">
                            后端 M3（L3/L4 时间聚合）未启用；dev:mock 模式可查看契约演示数据。
                        </p>
                    </div>
                </div>
            )}

            {!unavailable && windows.length === 0 && (
                <div className="mt-4 rounded-lg border border-dashed border-white/[0.08] px-3 py-4 text-center">
                    <p className="text-[12px] text-[#5A6778]">暂无窗口总结</p>
                </div>
            )}

            {windows.length > 0 && (
                <div className="mt-3 space-y-2">
                    {windows.map((window) => {
                        const meta = LEVEL_META[window.level]
                        return (
                            <div
                                key={window.windowId}
                                className="rounded-lg border border-white/[0.05] bg-white/[0.02] px-3 py-2.5"
                            >
                                <div className="flex items-center gap-2">
                                    <span className={cn('text-[11px] font-medium', meta.text)}>
                                        {meta.label}
                                    </span>
                                    {window.scores && (
                                        <span className={cn(
                                            'ml-auto font-mono text-[13px] font-semibold',
                                            window.scores.overall >= 80
                                                ? 'text-emerald-300'
                                                : window.scores.overall >= 60
                                                    ? 'text-amber-200'
                                                    : 'text-red-300',
                                        )}>
                                            {window.scores.overall}
                                        </span>
                                    )}
                                </div>
                                <p className="mt-1 font-mono text-[10px] text-[#5A6778]">
                                    {formatRange(window)}
                                </p>
                                {window.scores?.verdict && (
                                    <p className="mt-1.5 text-[12px] leading-5 text-[#C6D0DE]">
                                        {window.scores.verdict}
                                    </p>
                                )}
                                {window.scores?.reason && (
                                    <p className="mt-0.5 text-[11px] leading-4 text-[#8C99AC]">
                                        {window.scores.reason}
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
