import { useMemo } from 'react'
import {
    ChevronLeft,
    ChevronRight,
    Clock3,
    Expand,
    History,
    RotateCcw,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
    selectSelectedSnapshot,
    useTimeStore,
} from '@/stores/timeSlice'
import { FullscreenTimeline } from './FullscreenTimeline'
import { MiniTimeline } from './MiniTimeline'
import {
    chooseGranularity,
    countSnapshotsInViewport,
    formatUtc,
    getTimelineBounds,
} from './timelineMath'
import { useFullscreenTimeline } from '@/hooks/useFullscreenTimeline'
import { useControlPlaneStore } from '@/stores/controlPlaneSlice'

export function TimeTravelBar() {
    const fullscreen = useFullscreenTimeline()
    const snapshots = useTimeStore((state) => state.snapshots)
    const timestamp = useTimeStore((state) => state.timestamp)
    const mode = useTimeStore((state) => state.mode)
    const viewport = useTimeStore((state) => state.viewport)
    const selectedSnapshot = useTimeStore(selectSelectedSnapshot)
    const stepSnapshot = useTimeStore((state) => state.stepSnapshot)
    const returnToLatest = useTimeStore((state) => state.returnToLatest)

    const bounds = useMemo(() => getTimelineBounds(snapshots), [snapshots])
    const visibleCount = useMemo(
        () => countSnapshotsInViewport(snapshots, viewport),
        [snapshots, viewport],
    )
    const visibleSpan = Math.max(1_000, viewport.end - viewport.start)
    const totalSpan = Math.max(1_000, bounds.end - bounds.start)
    const zoomRatio = Math.max(1, totalSpan / visibleSpan)
    const granularity = chooseGranularity(visibleSpan, 720)
    const selectedIndex = selectedSnapshot
        ? snapshots.findIndex((snapshot) => snapshot.id === selectedSnapshot.id)
        : -1
    const hasAuthoritativeTime = Date.parse(timestamp) > 0
    const cluster = useControlPlaneStore((state) => state.cluster)
    const readyNodes = cluster.workers.filter(
        (worker) => worker.ready && worker.status === 'running',
    ).length
    const connectionDot =
        cluster.connectionStatus === 'connected'
            ? 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,.5)]'
            : cluster.connectionStatus === 'connecting'
                ? 'bg-amber-400 signal-pulse'
                : 'bg-red-400'

    return (
        <>
            <header className="relative z-30 shrink-0 border-b border-white/[0.07] bg-[#07090D]/95 px-3 py-1.5 shadow-[0_10px_40px_rgba(0,0,0,0.18)] backdrop-blur-xl md:h-[60px] md:px-4 md:py-1.5">
                <div className="flex h-full min-w-0 items-center gap-3 lg:gap-4">
                    <button
                        type="button"
                        onClick={fullscreen.show}
                        className="group flex w-[148px] min-w-0 shrink-0 items-center gap-2 rounded-lg px-1.5 py-1 text-left transition-colors hover:bg-white/[0.035] sm:w-[188px] lg:w-[210px]"
                        aria-label="打开时间切面探索器"
                    >
                        <span
                            className={
                                'flex h-7 w-7 shrink-0 items-center justify-center rounded-lg border transition-colors ' +
                                (mode === 'latest'
                                    ? 'border-emerald-400/20 bg-emerald-400/[0.08] text-emerald-300'
                                    : 'border-[#6E8BFF]/20 bg-[#6E8BFF]/10 text-[#9EB2FF]')
                            }
                        >
                            {mode === 'latest' ? (
                                <Clock3 className="h-3.5 w-3.5" />
                            ) : (
                                <History className="h-3.5 w-3.5" />
                            )}
                        </span>
                        <div className="min-w-0">
                            <div className="flex items-center gap-2">
                                <span className="flex items-center gap-1.5 text-[13px] text-[#596579]">
                                    <span className={connectionDot + ' h-1.5 w-1.5 rounded-full'} />
                                    <span className="hidden sm:inline">
                                        {cluster.connectionStatus === 'connected'
                                            ? '已连接'
                                            : cluster.connectionStatus === 'connecting'
                                                ? '连接中'
                                                : '未连接'}
                                    </span>
                                    <span className="font-mono text-[#748196]">
                                        {readyNodes} 节点
                                    </span>
                                </span>
                                <Badge
                                    variant="outline"
                                    className={
                                        'h-[18px] px-1.5 text-[12px] font-medium ' +
                                        (mode === 'latest'
                                            ? 'border-emerald-400/20 bg-emerald-400/[0.08] text-emerald-300'
                                            : 'border-[#6E8BFF]/20 bg-[#6E8BFF]/10 text-[#AFC0FF]')
                                    }
                                >
                                    {mode === 'latest' ? '最新' : '历史'}
                                </Badge>
                            </div>
                            <div className="mt-1 truncate font-mono text-[12px] font-medium tracking-[-0.01em] text-[#DDE4ED] sm:text-[14px] lg:text-xs">
                                {hasAuthoritativeTime ? formatUtc(timestamp, true) : '等待 Backend 权威时间'}
                                {hasAuthoritativeTime && (
                                    <span className="ml-1.5 text-[12px] font-normal text-[#596579]">
                                        UTC
                                    </span>
                                )}
                            </div>
                        </div>
                    </button>

                    <div className="min-w-0 flex-1">
                        <div className="mb-1 flex items-center justify-between gap-3 px-0.5 text-[12px] text-[#536074] sm:text-[13px]">
                            <span className="truncate font-mono">
                                {formatUtc(viewport.start)}
                            </span>
                            <span className="flex flex-wrap items-center gap-2">
                                <span>
                                    视窗 <strong className="font-mono font-medium text-[#8D9AAD]">{visibleCount}</strong>
                                </span>
                                <span className="h-2.5 w-px bg-white/[0.07]" />
                                <span>
                                    粒度 <strong className="font-medium text-[#8D9AAD]">{granularity.label}</strong>
                                </span>
                                <span className="h-2.5 w-px bg-white/[0.07]" />
                                <span>
                                    <strong className="font-mono font-medium text-[#8D9AAD]">
                                        {zoomRatio.toFixed(zoomRatio >= 10 ? 0 : 1)}×
                                    </strong>
                                </span>
                            </span>
                            <span className="truncate font-mono">
                                {formatUtc(viewport.end)}
                            </span>
                        </div>
                        <div className="h-8 overflow-hidden rounded-md border border-white/[0.055] bg-black/25 px-1 py-0.5">
                            <MiniTimeline />
                        </div>
                        <div className="mt-0.5 hidden items-center justify-center gap-1 text-[12px] text-[#465267] xl:flex">
                            滚轮缩放
                            <span>·</span>
                            拖动平移
                            <span>·</span>
                            点击任意位置切换回放切面
                        </div>
                    </div>

                    <div className="flex shrink-0 items-center gap-1">
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            onClick={() => stepSnapshot(-1)}
                            disabled={selectedIndex <= 0}
                            title="上一切面"
                            aria-label="上一切面"
                            className="h-8 w-8 text-[#778398] hover:bg-white/[0.055] hover:text-white disabled:opacity-25"
                        >
                            <ChevronLeft className="h-3.5 w-3.5" />
                        </Button>
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            onClick={() => stepSnapshot(1)}
                            disabled={
                                selectedIndex < 0 ||
                                selectedIndex >= snapshots.length - 1
                            }
                            title="下一切面"
                            aria-label="下一切面"
                            className="h-8 w-8 text-[#778398] hover:bg-white/[0.055] hover:text-white disabled:opacity-25"
                        >
                            <ChevronRight className="h-3.5 w-3.5" />
                        </Button>
                        {mode === 'historical' && (
                            <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                onClick={returnToLatest}
                                title="回到最新切面"
                                className="hidden h-8 px-2 text-[13px] text-[#93A1B5] hover:bg-white/[0.055] hover:text-white xl:flex"
                            >
                                <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                                最新
                            </Button>
                        )}
                        <div className="mx-0.5 hidden h-5 w-px bg-white/[0.07] sm:block" />
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            onClick={fullscreen.show}
                            title="打开时间切面探索器"
                            aria-label="打开时间切面探索器"
                            className="h-8 w-8 text-[#8C99AC] hover:bg-white/[0.055] hover:text-white"
                        >
                            <Expand className="h-3.5 w-3.5" />
                        </Button>
                    </div>
                </div>

                <span className="sr-only" aria-live="polite">
                    当前回放时间 {hasAuthoritativeTime ? `${formatUtc(timestamp, true)} UTC` : '等待 Backend 权威时间'}，
                    {mode === 'latest' ? '最新切面' : '历史切面'}
                </span>
            </header>

            <FullscreenTimeline
                open={fullscreen.open}
                onOpenChange={fullscreen.setOpen}
            />
        </>
    )
}
