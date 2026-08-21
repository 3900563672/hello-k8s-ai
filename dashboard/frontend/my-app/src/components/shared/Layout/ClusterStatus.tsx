import { useEffect } from 'react'
import {
    Check,
    LockKeyhole,
    PlugZap,
    RefreshCw,
    Send,
    ServerCog,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from '@/components/ui/popover'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
    onlineWorkerCount,
    useControlPlaneStore,
} from '@/stores/controlPlaneSlice'
import type {
    ClusterNodeStatus,
    ConnectionStatus,
} from '@/types/control-plane.types'
import { cn } from '@/lib/utils'
import { useTimeStore } from '@/stores/timeSlice'

const connectionCopy: Record<
    ConnectionStatus,
    { label: string; dot: string; icon: string }
> = {
    connected: {
        label: '已连接',
        dot: 'bg-emerald-400 shadow-[0_0_10px_rgba(52,211,153,.55)]',
        icon: 'text-[#79ADFF]',
    },
    connecting: {
        label: '连接中',
        dot: 'bg-amber-400 signal-pulse',
        icon: 'text-amber-300',
    },
    disconnected: {
        label: '未连接',
        dot: 'bg-red-400 shadow-[0_0_9px_rgba(248,113,113,.45)]',
        icon: 'text-[#596477]',
    },
}

const nodeStatusCopy: Record<
    ClusterNodeStatus,
    { label: string; dot: string; text: string }
> = {
    running: {
        label: '运行中',
        dot: 'bg-emerald-400',
        text: 'text-emerald-300',
    },
    offline: {
        label: '离线',
        dot: 'bg-red-400',
        text: 'text-red-300',
    },
    unknown: {
        label: '未知',
        dot: 'bg-[#657184]',
        text: 'text-[#778397]',
    },
}

export function ClusterStatus() {
    const cluster = useControlPlaneStore((state) => state.cluster)
    const refreshPhase = useControlPlaneStore((state) => state.refreshPhase)
    const distributionPhase = useControlPlaneStore(
        (state) => state.distributionPhase,
    )
    const distributionReceipt = useControlPlaneStore(
        (state) => state.distributionReceipt,
    )
    const lastError = useControlPlaneStore((state) => state.lastError)
    const refreshCluster = useControlPlaneStore((state) => state.refreshCluster)
    const distributeConfig = useControlPlaneStore(
        (state) => state.distributeConfig,
    )
    const clearDistributionFeedback = useControlPlaneStore(
        (state) => state.clearDistributionFeedback,
    )
    const timeMode = useTimeStore((state) => state.mode)

    const onlineWorkers = onlineWorkerCount(cluster)
    const connection = connectionCopy[cluster.connectionStatus]
    const canDistribute =
        timeMode === 'latest' && cluster.connectionStatus === 'connected' && onlineWorkers > 0

    useEffect(() => {
        if (distributionPhase !== 'success') return
        const timer = window.setTimeout(clearDistributionFeedback, 1800)
        return () => window.clearTimeout(timer)
    }, [clearDistributionFeedback, distributionPhase])

    const handleDistribute = async () => {
        await distributeConfig()
    }

    return (
        <Popover>
            <PopoverTrigger asChild>
                <button
                    type="button"
                    className="group relative flex h-[74px] w-full flex-col items-center justify-center gap-1 rounded-xl border border-transparent text-[#6E7A8D] outline-none transition duration-150 hover:border-white/[0.06] hover:bg-white/[0.035] focus-visible:border-[#5B8CFF]/45 focus-visible:ring-2 focus-visible:ring-[#5B8CFF]/20"
                    aria-label={`${connection.label}，${onlineWorkers}/${cluster.workers.length} 个工作节点在线，打开集群详情`}
                >
                    <span className="relative flex h-7 w-8 items-center justify-center">
                        <PlugZap
                            className={cn(
                                'h-[19px] w-[19px] transition-colors group-hover:text-[#B6C3D6]',
                                connection.icon,
                            )}
                        />
                        <span
                            className={cn(
                                'absolute right-0.5 top-0 h-2 w-2 rounded-full ring-[3px] ring-[#080B11]',
                                connection.dot,
                            )}
                        />
                    </span>
                    <span
                        className={cn(
                            'font-mono text-[12px] font-medium tabular-nums tracking-tight',
                            onlineWorkers === 0 ? 'text-red-300' : 'text-[#8D9AAD]',
                        )}
                    >
                        {onlineWorkers}/{cluster.workers.length}
                    </span>
                    <span className="sr-only">{connection.label}</span>
                </button>
            </PopoverTrigger>

            <PopoverContent
                side="right"
                align="end"
                sideOffset={14}
                collisionPadding={16}
                className="w-[320px] border-white/[0.09] bg-[#0C1119]/98 p-0 text-[#E7EDF6] backdrop-blur-2xl"
            >
                <div className="border-b border-white/[0.07] px-4 py-3.5">
                    <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                            <div className="flex items-center gap-2 text-xs font-semibold text-[#EDF3FA]">
                                <span className={cn('h-2 w-2 rounded-full', connection.dot)} />
                                {connection.label}
                            </div>
                            <p className="mt-1 truncate font-mono text-[13px] text-[#667286]">
                                {cluster.context} · {cluster.version}
                            </p>
                        </div>
                        <div className="min-w-0 text-right">
                            <div className="truncate text-[12px] font-medium text-[#9AA7B9]">
                                {cluster.name}
                            </div>
                            <div className="mt-1 truncate text-[13px] text-[#596579]">
                                {cluster.provider}
                            </div>
                        </div>
                    </div>
                </div>

                <div className="border-b border-white/[0.07] px-4 py-3">
                    <ClusterSummaryRow
                        icon={<LockKeyhole className="h-3 w-3" />}
                        label="控制平面"
                        value={cluster.controlPlane.ready ? '已连接 · 只读' : '未就绪 · 只读'}
                        accent={cluster.controlPlane.ready ? 'text-emerald-300' : 'text-amber-300'}
                    />
                    <ClusterSummaryRow
                        icon={<ServerCog className="h-3 w-3" />}
                        label="工作节点"
                        value={`${onlineWorkers}/${cluster.workers.length} 可用`}
                        accent="text-[#82B4FF]"
                    />
                </div>

                <div className="px-2 py-2">
                    <div className="flex items-center justify-between px-2 pb-1.5 pt-0.5">
                        <span className="text-[13px] font-medium uppercase tracking-[0.14em] text-[#566174]">
                            Worker nodes
                        </span>
                        <span className="font-mono text-[12px] text-[#465267]">
                            {formatCheckedAt(cluster.checkedAt)} UTC
                        </span>
                    </div>
                    <ScrollArea className="h-[228px]">
                        <div className="space-y-0.5 pr-2">
                            {cluster.workers.map((node) => {
                                const status = nodeStatusCopy[node.status]
                                return (
                                    <div
                                        key={node.id}
                                        className="group/node flex h-8 items-center gap-2 rounded-lg px-2 transition-colors hover:bg-white/[0.035]"
                                    >
                                        <span className={cn('h-1.5 w-1.5 shrink-0 rounded-full', status.dot)} />
                                        <span className="min-w-0 flex-1 truncate font-mono text-[12px] text-[#B7C2D1]">
                                            {node.name}
                                        </span>
                                        <span className="hidden font-mono text-[12px] text-[#4E5A6C] group-hover/node:inline">
                                            {node.zone}
                                        </span>
                                        <span className={cn('shrink-0 text-[13px]', status.text)}>
                                            {status.label}
                                        </span>
                                    </div>
                                )
                            })}
                        </div>
                    </ScrollArea>
                </div>

                {lastError && (
                    <div
                        role="alert"
                        className="mx-4 mb-3 rounded-lg border border-red-400/15 bg-red-400/[0.06] px-3 py-2 text-[12px] leading-4 text-red-200"
                    >
                        {lastError}
                    </div>
                )}

                {timeMode === 'historical' && (
                    <div className="mx-4 mb-3 rounded-lg border border-[#7D8FFF]/15 bg-[#7D8FFF]/[0.055] px-3 py-2 text-[12px] leading-4 text-[#9DA9DB]">
                        历史回放为只读；回到最新切面后才能核验当前配置。
                    </div>
                )}

                <div className="flex items-center gap-2 border-t border-white/[0.07] px-3 py-3">
                    <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => void refreshCluster()}
                        disabled={refreshPhase === 'pending'}
                        className="h-8 gap-1.5 px-2.5 text-[12px] text-[#8793A5] hover:bg-white/[0.05] hover:text-white"
                    >
                        <RefreshCw
                            className={cn(
                                'h-3.5 w-3.5',
                                refreshPhase === 'pending' && 'animate-spin',
                            )}
                        />
                        刷新
                    </Button>
                    <Button
                        type="button"
                        size="sm"
                        onClick={() => void handleDistribute()}
                        disabled={!canDistribute || distributionPhase === 'pending'}
                        title={timeMode === 'historical' ? '历史回放模式下不可核验当前配置' : undefined}
                        className={cn(
                            'h-8 min-w-0 flex-1 gap-1.5 bg-[#5B8CFF] px-3 text-[12px] text-white shadow-[0_0_24px_rgba(91,140,255,.14)] hover:bg-[#70A0FF] disabled:bg-[#222A36] disabled:text-[#596477]',
                            distributionPhase === 'success' &&
                                'bg-emerald-500 hover:bg-emerald-500',
                        )}
                    >
                        {distributionPhase === 'pending' ? (
                            <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                        ) : distributionPhase === 'success' ? (
                            <Check className="h-3.5 w-3.5" />
                        ) : (
                            <Send className="h-3.5 w-3.5" />
                        )}
                        {distributionPhase === 'success'
                            ? `已核验 ${distributionReceipt?.resources.total ?? 0} 项`
                            : '核验配置'}
                    </Button>
                </div>
            </PopoverContent>
        </Popover>
    )
}

function ClusterSummaryRow({
    icon,
    label,
    value,
    accent,
}: {
    icon: React.ReactNode
    label: string
    value: string
    accent: string
}) {
    return (
        <div className="flex h-7 items-center gap-2 text-[12px]">
            <span className={cn('flex w-4 justify-center', accent)}>{icon}</span>
            <span className="flex-1 text-[#778397]">{label}</span>
            <span className={cn('font-mono font-medium tabular-nums', accent)}>{value}</span>
        </div>
    )
}

function formatCheckedAt(timestamp: string) {
    const value = new Date(timestamp)
    if (Number.isNaN(value.getTime())) return '--:--:--'
    return new Intl.DateTimeFormat('zh-CN', {
        timeZone: 'UTC',
        hour12: false,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
    }).format(value)
}
