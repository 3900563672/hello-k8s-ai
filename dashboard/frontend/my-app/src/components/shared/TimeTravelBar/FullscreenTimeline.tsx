import { useEffect, useMemo, useState, type FormEvent } from 'react'
import {
    Activity,
    ArrowLeft,
    ArrowRight,
    CalendarClock,
    Clock3,
    Cpu,
    Database,
    History,
    RotateCcw,
    Server,
    Users,
    Waypoints,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetHeader,
    SheetTitle,
} from '@/components/ui/sheet'
import {
    selectSelectedSnapshot,
    useTimeStore,
} from '@/stores/timeSlice'
import type {
    SnapshotDomain,
    SnapshotSeverity,
} from '@/types/time.types'
import { TimelineChart } from './TimelineChart'
import {
    countByTrigger,
    countSnapshotsInViewport,
    formatDuration,
    formatUtc,
    getTimelineBounds,
    parseUtcDateTimeInput,
    toUtcDateTimeInput,
    triggerLabel,
} from './timelineMath'

interface FullscreenTimelineProps {
    open: boolean
    onOpenChange: (open: boolean) => void
}

const rangePresets = [
    { label: '1 分钟', duration: 60_000 },
    { label: '15 分钟', duration: 15 * 60_000 },
    { label: '1 小时', duration: 60 * 60_000 },
    { label: '1 天', duration: 24 * 60 * 60_000 },
    { label: '7 天', duration: 7 * 24 * 60 * 60_000 },
    { label: '全部', duration: null },
] as const

const domainLabels: Record<SnapshotDomain, string> = {
    scheduler: '调度器',
    configuration: '配置',
    capacity: '容量',
    runtime: '运行态',
}

const severityStyles: Record<SnapshotSeverity, string> = {
    normal: 'border-[#6E8BFF]/20 bg-[#6E8BFF]/10 text-[#AFC0FF]',
    attention: 'border-amber-400/20 bg-amber-400/10 text-amber-200',
    critical: 'border-red-400/20 bg-red-400/10 text-red-200',
}

export function FullscreenTimeline({
    open,
    onOpenChange,
}: FullscreenTimelineProps) {
    const snapshots = useTimeStore((state) => state.snapshots)
    const timestamp = useTimeStore((state) => state.timestamp)
    const mode = useTimeStore((state) => state.mode)
    const viewport = useTimeStore((state) => state.viewport)
    const selectedSnapshot = useTimeStore(selectSelectedSnapshot)
    const jumpToTimestamp = useTimeStore((state) => state.jumpToTimestamp)
    const stepSnapshot = useTimeStore((state) => state.stepSnapshot)
    const returnToLatest = useTimeStore((state) => state.returnToLatest)
    const focusDuration = useTimeStore((state) => state.focusDuration)

    // 初始 timestamp 为 1970 纪元占位；权威时间到达前禁用精确跳转，避免展示纪元时间
    const hasAuthoritativeTime = Date.parse(timestamp) > 0
    const [jumpValue, setJumpValue] = useState(() =>
        hasAuthoritativeTime ? toUtcDateTimeInput(timestamp) : '',
    )
    const [jumpError, setJumpError] = useState('')

    useEffect(() => {
        setJumpValue(hasAuthoritativeTime ? toUtcDateTimeInput(timestamp) : '')
        setJumpError('')
    }, [hasAuthoritativeTime, timestamp])

    const bounds = useMemo(() => getTimelineBounds(snapshots), [snapshots])
    const visibleCount = useMemo(
        () => countSnapshotsInViewport(snapshots, viewport),
        [snapshots, viewport],
    )
    const timeCount = useMemo(
        () => countByTrigger(snapshots, 'time'),
        [snapshots],
    )
    const eventCount = snapshots.length - timeCount
    const selectedIndex = selectedSnapshot
        ? snapshots.findIndex((snapshot) => snapshot.id === selectedSnapshot.id)
        : -1
    const visibleSpan = Math.max(1_000, viewport.end - viewport.start)
    const totalSpan = Math.max(1_000, bounds.end - bounds.start)
    const zoomRatio = Math.max(1, totalSpan / visibleSpan)

    const submitJump = (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault()
        const target = parseUtcDateTimeInput(jumpValue)
        if (target === null) {
            setJumpError('请输入有效的 UTC 时间')
            return
        }
        if (target < bounds.start || target > bounds.end) {
            setJumpError('目标时间不在可回放范围内')
            return
        }
        jumpToTimestamp(target)
        setJumpError('')
    }

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent
                side="bottom"
                className="flex h-[94dvh] max-h-[94dvh] flex-col gap-0 overflow-hidden border-t border-white/[0.09] bg-[#06080C] p-0 text-[#EDF2F8] shadow-[0_-30px_100px_rgba(0,0,0,0.72)]"
            >
                <SheetHeader className="shrink-0 border-b border-white/[0.07] px-5 py-4 pr-14 text-left lg:px-7">
                    <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                        <div className="flex min-w-0 items-center gap-3">
                            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-[#6E8BFF]/20 bg-[#6E8BFF]/10">
                                <Waypoints className="h-4 w-4 text-[#9EB2FF]" />
                            </div>
                            <div className="min-w-0">
                                <div className="flex items-center gap-2">
                                    <SheetTitle className="text-sm font-semibold tracking-[-0.01em] text-[#F2F5F9]">
                                        时间切面探索器
                                    </SheetTitle>
                                    <Badge
                                        variant="outline"
                                        className={
                                            mode === 'latest'
                                                ? 'h-5 border-emerald-400/20 bg-emerald-400/10 px-1.5 text-[9px] font-medium text-emerald-300'
                                                : 'h-5 border-[#6E8BFF]/20 bg-[#6E8BFF]/10 px-1.5 text-[9px] font-medium text-[#AFC0FF]'
                                        }
                                    >
                                        {mode === 'latest' ? '最新切面' : '历史回放'}
                                    </Badge>
                                </div>
                                <SheetDescription className="mt-1 truncate text-[11px] text-[#667286]">
                                    缩放范围可从数月收敛到秒级；选中切面将成为全站数据回显时间。
                                </SheetDescription>
                            </div>
                        </div>

                        <div className="flex flex-wrap items-center gap-2">
                            <div className="mr-1 hidden items-center gap-3 text-[10px] text-[#6D7889] xl:flex">
                                <span>
                                    全量 <strong className="font-mono font-medium text-[#B9C4D3]">{snapshots.length}</strong>
                                </span>
                                <span>
                                    视窗 <strong className="font-mono font-medium text-[#B9C4D3]">{visibleCount}</strong>
                                </span>
                                <span>
                                    缩放 <strong className="font-mono font-medium text-[#B9C4D3]">{zoomRatio.toFixed(zoomRatio >= 10 ? 0 : 1)}×</strong>
                                </span>
                            </div>
                            <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                onClick={() => stepSnapshot(-1)}
                                disabled={selectedIndex <= 0}
                                className="h-8 border-white/[0.08] bg-white/[0.025] px-2.5 text-[11px] text-[#9AA6B6] hover:bg-white/[0.06] hover:text-white disabled:opacity-30"
                            >
                                <ArrowLeft className="mr-1.5 h-3.5 w-3.5" />
                                上一切面
                            </Button>
                            <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                onClick={() => stepSnapshot(1)}
                                disabled={
                                    selectedIndex < 0 ||
                                    selectedIndex >= snapshots.length - 1
                                }
                                className="h-8 border-white/[0.08] bg-white/[0.025] px-2.5 text-[11px] text-[#9AA6B6] hover:bg-white/[0.06] hover:text-white disabled:opacity-30"
                            >
                                下一切面
                                <ArrowRight className="ml-1.5 h-3.5 w-3.5" />
                            </Button>
                            {mode === 'historical' && (
                                <Button
                                    type="button"
                                    size="sm"
                                    onClick={returnToLatest}
                                    className="h-8 bg-[#E8EDF5] px-3 text-[11px] font-medium text-[#0B0E13] hover:bg-white"
                                >
                                    <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                                    回到最新
                                </Button>
                            )}
                        </div>
                    </div>
                </SheetHeader>

                <div className="min-h-0 flex-1 overflow-y-auto p-4 lg:overflow-hidden lg:p-5">
                    <div className="grid min-h-full gap-4 lg:h-full lg:grid-cols-[minmax(0,1fr)_330px] xl:grid-cols-[minmax(0,1fr)_360px]">
                        <section className="flex min-h-[500px] min-w-0 flex-col overflow-hidden rounded-2xl border border-white/[0.07] bg-[#090C12] lg:min-h-0">
                            <div className="flex flex-col gap-3 border-b border-white/[0.06] px-4 py-3.5 xl:flex-row xl:items-center xl:justify-between">
                                <div>
                                    <div className="flex items-center gap-2 text-xs font-medium text-[#CFD7E3]">
                                        <Activity className="h-3.5 w-3.5 text-[#7F96EF]" />
                                        调度活动密度
                                    </div>
                                    <div className="mt-1 font-mono text-[9px] text-[#566174]">
                                        {formatUtc(viewport.start)} — {formatUtc(viewport.end)} UTC
                                    </div>
                                </div>
                                <div className="flex flex-wrap items-center gap-1 rounded-lg border border-white/[0.055] bg-black/20 p-1">
                                    {rangePresets.map((preset) => {
                                        const active =
                                            preset.duration === null
                                                ? zoomRatio <= 1.01
                                                : Math.abs(
                                                      visibleSpan - preset.duration,
                                                  ) /
                                                      preset.duration <
                                                  0.05
                                        return (
                                            <button
                                                key={preset.label}
                                                type="button"
                                                onClick={() =>
                                                    focusDuration(preset.duration)
                                                }
                                                className={
                                                    'h-6 rounded-md px-2 text-[9px] transition-colors ' +
                                                    (active
                                                        ? 'bg-white/[0.09] text-[#E8EDF5]'
                                                        : 'text-[#667286] hover:bg-white/[0.045] hover:text-[#AEB9C8]')
                                                }
                                            >
                                                {preset.label}
                                            </button>
                                        )
                                    })}
                                </div>
                            </div>

                            <div className="min-h-[350px] flex-1 px-2 pb-1 pt-2 lg:min-h-0">
                                <TimelineChart variant="explorer" />
                            </div>

                            <div className="flex flex-wrap items-center justify-between gap-3 border-t border-white/[0.055] px-4 py-3 text-[9px] text-[#596579]">
                                <div className="flex items-center gap-4">
                                    <LegendDot color="#6E8BFF" label={'时间驱动 ' + timeCount} />
                                    <LegendDot color="#F0A33B" label={'事件驱动 ' + eventCount} />
                                    <span className="hidden sm:inline">
                                        柱高表示该时间桶内的切面密度与峰值权重
                                    </span>
                                </div>
                                <span>滚轮缩放 · 拖动平移 · 单击任意位置回放</span>
                            </div>
                        </section>

                        <aside className="min-h-0 space-y-4 lg:overflow-y-auto lg:pr-1">
                            <section className="rounded-2xl border border-white/[0.07] bg-[#090C12] p-4">
                                <div className="flex items-center gap-2 text-xs font-medium text-[#CFD7E3]">
                                    <CalendarClock className="h-3.5 w-3.5 text-[#7F96EF]" />
                                    精确跳转
                                </div>
                                <p className="mt-1.5 text-[10px] leading-4 text-[#657184]">
                                    输入 UTC 时间后，定位到该时刻之前最后一个有效切面。
                                </p>
                                <form onSubmit={submitJump} className="mt-3 space-y-2">
                                    <Input
                                        type="datetime-local"
                                        step="0.001"
                                        value={jumpValue}
                                        onChange={(event) =>
                                            setJumpValue(event.target.value)
                                        }
                                        disabled={!hasAuthoritativeTime}
                                        placeholder={hasAuthoritativeTime ? undefined : '等待数据…'}
                                        aria-label="UTC 精确跳转时间"
                                        className="h-9 border-white/[0.08] bg-white/[0.025] font-mono text-[10px] text-[#DCE4EE] placeholder:text-[#596579] [color-scheme:dark] focus-visible:border-[#6E8BFF]/50 focus-visible:ring-[#6E8BFF]/15 disabled:opacity-50"
                                    />
                                    {!hasAuthoritativeTime && (
                                        <p className="text-[10px] text-[#596579]">
                                            等待 Backend 权威时间后可用精确跳转
                                        </p>
                                    )}
                                    {jumpError && (
                                        <p className="text-[10px] text-amber-300">
                                            {jumpError}
                                        </p>
                                    )}
                                    <Button
                                        type="submit"
                                        variant="outline"
                                        disabled={!hasAuthoritativeTime}
                                        className="h-8 w-full border-white/[0.08] bg-white/[0.03] text-[10px] text-[#B7C2D0] hover:bg-white/[0.07] hover:text-white disabled:opacity-40"
                                    >
                                        <Clock3 className="mr-1.5 h-3.5 w-3.5" />
                                        定位并回放
                                    </Button>
                                </form>
                            </section>

                            <section className="overflow-hidden rounded-2xl border border-white/[0.07] bg-[#090C12]">
                                <div className="border-b border-white/[0.06] px-4 py-3">
                                    <div className="flex items-center justify-between gap-3">
                                        <div className="flex items-center gap-2 text-xs font-medium text-[#CFD7E3]">
                                            <History className="h-3.5 w-3.5 text-[#7F96EF]" />
                                            当前回放切面
                                        </div>
                                        {selectedSnapshot && (
                                            <Badge
                                                variant="outline"
                                                className={
                                                    'h-5 px-1.5 text-[9px] ' +
                                                    severityStyles[
                                                        selectedSnapshot.severity
                                                    ]
                                                }
                                            >
                                                {triggerLabel(
                                                    selectedSnapshot.trigger,
                                                )}
                                            </Badge>
                                        )}
                                    </div>
                                </div>

                                {selectedSnapshot ? (
                                    <div className="p-4">
                                        <div className="font-mono text-[10px] text-[#8FA2B9]">
                                            {formatUtc(
                                                selectedSnapshot.timestamp,
                                                true,
                                            )}{' '}
                                            UTC
                                        </div>
                                        <h3 className="mt-2 text-sm font-semibold leading-5 text-[#EDF2F8]">
                                            {selectedSnapshot.title}
                                        </h3>
                                        <p className="mt-2 text-[11px] leading-[1.65] text-[#758195]">
                                            {selectedSnapshot.summary}
                                        </p>

                                        <div className="mt-4 grid grid-cols-2 gap-2">
                                            <ImpactMetric
                                                icon={Users}
                                                label="租户"
                                                value={selectedSnapshot.impact.tenants}
                                            />
                                            <ImpactMetric
                                                icon={Server}
                                                label="节点"
                                                value={selectedSnapshot.impact.nodes}
                                            />
                                            <ImpactMetric
                                                icon={Cpu}
                                                label="模型"
                                                value={selectedSnapshot.impact.models}
                                            />
                                            <ImpactMetric
                                                icon={Database}
                                                label="变更"
                                                value={selectedSnapshot.impact.changes}
                                            />
                                        </div>

                                        <div className="mt-4 space-y-2 border-t border-white/[0.055] pt-3 text-[10px]">
                                            <DetailRow
                                                label="领域"
                                                value={
                                                    domainLabels[
                                                        selectedSnapshot.domain
                                                    ]
                                                }
                                            />
                                            <DetailRow
                                                label="来源"
                                                value={selectedSnapshot.source}
                                                mono
                                            />
                                            <DetailRow
                                                label="序号"
                                                value={
                                                    String(selectedIndex + 1) +
                                                    ' / ' +
                                                    snapshots.length
                                                }
                                                mono
                                            />
                                        </div>

                                        <div className="mt-4 flex flex-wrap gap-1.5">
                                            {selectedSnapshot.tags.map((tag) => (
                                                <span
                                                    key={tag}
                                                    className="rounded-md border border-white/[0.06] bg-white/[0.025] px-1.5 py-1 text-[9px] text-[#718096]"
                                                >
                                                    {tag}
                                                </span>
                                            ))}
                                        </div>

                                        <div className="mt-4 rounded-xl border border-[#6E8BFF]/15 bg-[#6E8BFF]/[0.055] p-3">
                                            <div className="text-[9px] font-medium text-[#9CAFFF]">
                                                全站回放锚点
                                            </div>
                                            <code className="mt-1.5 block break-all text-[9px] leading-4 text-[#7587A1]">
                                                {selectedSnapshot.id}
                                            </code>
                                            <p className="mt-2 text-[9px] leading-4 text-[#637189]">
                                                后续页面可使用 effectiveAt 与此切面 ID
                                                获取一致的历史数据。
                                            </p>
                                        </div>
                                    </div>
                                ) : (
                                    <div className="px-4 py-10 text-center text-[10px] text-[#596579]">
                                        暂无可回放切面
                                    </div>
                                )}
                            </section>

                            <section className="rounded-2xl border border-white/[0.07] bg-[#090C12] p-4">
                                <div className="flex items-center justify-between text-[10px]">
                                    <span className="text-[#657184]">当前视窗跨度</span>
                                    <span className="font-mono text-[#AAB6C5]">
                                        {formatDuration(visibleSpan)}
                                    </span>
                                </div>
                                <div className="mt-2 flex items-center justify-between text-[10px]">
                                    <span className="text-[#657184]">完整数据跨度</span>
                                    <span className="font-mono text-[#AAB6C5]">
                                        {formatDuration(totalSpan)}
                                    </span>
                                </div>
                            </section>
                        </aside>
                    </div>
                </div>
            </SheetContent>
        </Sheet>
    )
}

function LegendDot({ color, label }: { color: string; label: string }) {
    return (
        <span className="inline-flex items-center gap-1.5">
            <span
                className="h-1.5 w-1.5 rounded-[2px]"
                style={{ backgroundColor: color }}
            />
            {label}
        </span>
    )
}

function DetailRow({
    label,
    value,
    mono = false,
}: {
    label: string
    value: string
    mono?: boolean
}) {
    return (
        <div className="flex items-start justify-between gap-3">
            <span className="shrink-0 text-[#5E6A7C]">{label}</span>
            <span
                className={
                    'break-all text-right text-[#9EABBC] ' +
                    (mono ? 'font-mono text-[9px]' : '')
                }
            >
                {value}
            </span>
        </div>
    )
}

function ImpactMetric({
    icon: Icon,
    label,
    value,
}: {
    icon: typeof Users
    label: string
    value: number
}) {
    return (
        <div className="rounded-xl border border-white/[0.055] bg-white/[0.018] p-2.5">
            <div className="flex items-center gap-1.5 text-[9px] text-[#5F6B7E]">
                <Icon className="h-3 w-3" />
                {label}
            </div>
            <div className="mt-1.5 font-mono text-xs font-medium text-[#C4CFDC]">
                {value}
            </div>
        </div>
    )
}
