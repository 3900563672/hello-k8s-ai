import { FilePenLine, Gauge, Play } from 'lucide-react'
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip'
import {
    canRunTest,
    useControlPlaneStore,
} from '@/stores/controlPlaneSlice'
import { useTimeStore } from '@/stores/timeSlice'
import { cn } from '@/lib/utils'

export function ExecutionControls() {
    const cluster = useControlPlaneStore((state) => state.cluster)
    const executionMode = useControlPlaneStore((state) => state.executionMode)
    const executionPhase = useControlPlaneStore((state) => state.executionPhase)
    const setExecutionMode = useControlPlaneStore((state) => state.setExecutionMode)
    const simulationRatePhase = useControlPlaneStore((state) => state.simulationRatePhase)
    const setSimulationRate = useControlPlaneStore((state) => state.setSimulationRate)
    const timeMode = useTimeStore((state) => state.mode)

    const isHistorical = timeMode === 'historical'
    const clusterCanTest = canRunTest(cluster)
    const testDisabled = isHistorical || !clusterCanTest
    const testDisabledReason = isHistorical
        ? '历史回放为只读，请先回到最新切面'
        : !cluster.simulationRunSupported
            ? '当前 Backend 未开放真实仿真运行控制'
            : '需要至少一个在线工作节点'

    const rateDisabled =
        isHistorical ||
        cluster.connectionStatus !== 'connected' ||
        !cluster.simulationRateSupported ||
        (Boolean(cluster.clockResourceVersion) && !cluster.clockConverged) ||
        simulationRatePhase === 'pending'
    const rateOptions = [1, 2, 5, 10, 20].includes(cluster.clockRate)
        ? [1, 2, 5, 10, 20]
        : [...[1, 2, 5, 10, 20], cluster.clockRate].sort((left, right) => left - right)
    const rateDisabledReason = isHistorical
        ? '历史回放为只读，请先回到最新切面'
        : !cluster.simulationRateSupported
            ? '当前 Backend 未开放 Simulator 倍速写入'
            : cluster.connectionStatus !== 'connected'
                ? '等待 Kubernetes cache 就绪'
                : !cluster.clockConverged
                    ? 'Controller 正在同步上一条倍速配置'
                    : '倍速更新正在提交'

    const status = getExecutionStatus({
        isHistorical,
        executionMode,
        executionPhase,
    })

    const startTestRun = () => {
        if (executionMode === 'test') return
        setExecutionMode('test')
    }

    return (
        <div className="flex w-full flex-col items-center gap-1.5 py-2">
            <div
                className="grid grid-cols-2 gap-1 rounded-xl border border-white/[0.055] bg-black/20 p-1"
                aria-label="执行模式"
            >
                <Tooltip>
                    <TooltipTrigger asChild>
                        <span className={cn(isHistorical && 'cursor-not-allowed')}>
                            <button
                                type="button"
                                disabled={isHistorical}
                                aria-pressed={executionMode === 'apply'}
                                onClick={() => setExecutionMode('apply')}
                                className={cn(
                                    'flex h-7 w-6 items-center justify-center rounded-lg text-[#687487] outline-none transition duration-150 hover:bg-white/[0.055] hover:text-[#C7D2E1] focus-visible:ring-2 focus-visible:ring-[#5B8CFF]/35 disabled:cursor-not-allowed disabled:opacity-45',
                                    executionMode === 'apply' &&
                                        'bg-[#5B8CFF] text-white shadow-[0_0_18px_rgba(91,140,255,.2)] hover:bg-[#6A97F8] hover:text-white',
                                )}
                            >
                                <FilePenLine className="h-3.5 w-3.5" />
                                <span className="sr-only">应用模式</span>
                            </button>
                        </span>
                    </TooltipTrigger>
                    <TooltipContent
                        side="right"
                        sideOffset={12}
                        className="border border-white/[0.08] bg-[#111722] text-[#DDE5F0]"
                    >
                        {isHistorical ? '历史回放为只读' : '应用：仅写入 CRD'}
                    </TooltipContent>
                </Tooltip>

                <Tooltip>
                    <TooltipTrigger asChild>
                        <span className={cn(testDisabled && 'cursor-not-allowed')}>
                            <button
                                type="button"
                                disabled={testDisabled}
                                aria-pressed={executionMode === 'test'}
                                onClick={startTestRun}
                                className={cn(
                                    'flex h-7 w-6 items-center justify-center rounded-lg text-[#687487] outline-none transition duration-150 hover:bg-white/[0.055] hover:text-[#C7D2E1] focus-visible:ring-2 focus-visible:ring-[#5B8CFF]/35 disabled:cursor-not-allowed disabled:text-[#394252]',
                                    executionMode === 'test' &&
                                        'bg-amber-400/15 text-amber-300 shadow-[0_0_18px_rgba(251,191,36,.12)] hover:bg-amber-400/20 hover:text-amber-200',
                                )}
                            >
                                <Play className="h-3.5 w-3.5 fill-current" />
                                <span className="sr-only">测试运行模式</span>
                            </button>
                        </span>
                    </TooltipTrigger>
                    <TooltipContent
                        side="right"
                        sideOffset={12}
                        className="border border-white/[0.08] bg-[#111722] text-[#DDE5F0]"
                    >
                        {testDisabled ? testDisabledReason : '测试：触发调度模拟'}
                    </TooltipContent>
                </Tooltip>
            </div>

            <Tooltip>
                <TooltipTrigger asChild>
                    <div
                        className={cn(
                            'flex h-7 w-[52px] items-center justify-center gap-1 rounded-lg border border-white/[0.055] bg-black/20 px-1 text-[#7C899C]',
                            rateDisabled && 'cursor-not-allowed opacity-45',
                        )}
                    >
                        <Gauge className="h-3 w-3 shrink-0" />
                        <select
                            aria-label="Simulator 时间倍速"
                            value={cluster.clockRate}
                            disabled={rateDisabled}
                            onChange={(event) => void setSimulationRate(Number(event.target.value))}
                            className="w-[28px] appearance-none bg-transparent text-center font-mono text-[13px] font-semibold text-[#AEB9C9] outline-none disabled:cursor-not-allowed"
                        >
                            {rateOptions.map((rate) => (
                                <option key={rate} value={rate} className="bg-[#111722] text-[#DDE5F0]">
                                    {rate}x
                                </option>
                            ))}
                        </select>
                    </div>
                </TooltipTrigger>
                <TooltipContent
                    side="right"
                    sideOffset={12}
                    className="max-w-64 border border-white/[0.08] bg-[#111722] text-[#DDE5F0]"
                >
                    {rateDisabled
                        ? rateDisabledReason
                        : cluster.clockConverged
                            ? `Simulator 已按 ${cluster.clockAppliedRate}x 运行（${cluster.clockSynchronizedInstances}/${cluster.clockTotalInstances} 个实例）`
                            : `目标 ${cluster.clockRate}x，Controller 正在同步实例`}
                </TooltipContent>
            </Tooltip>

            <div className="flex max-w-[60px] items-center justify-center gap-1.5">
                <span className={cn('h-1.5 w-1.5 shrink-0 rounded-full', status.dot)} />
                <span className="truncate text-[13px] font-medium text-[#7C899C]">
                    {status.label}
                </span>
            </div>
        </div>
    )
}

function getExecutionStatus({
    isHistorical,
    executionMode,
    executionPhase,
}: {
    isHistorical: boolean
    executionMode: 'apply' | 'test'
    executionPhase: 'standby' | 'running' | 'error'
}) {
    if (isHistorical) {
        return {
            label: '历史只读',
            dot: 'bg-[#7D8FFF] shadow-[0_0_8px_rgba(125,143,255,.45)]',
        }
    }
    if (executionPhase === 'error') {
        return {
            label: '错误',
            dot: 'bg-red-400 shadow-[0_0_8px_rgba(248,113,113,.45)]',
        }
    }
    if (executionMode === 'test') {
        return {
            label: '测试运行',
            dot: 'bg-amber-400 signal-pulse',
        }
    }
    return {
        label: '待命中',
        dot: 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,.42)]',
    }
}
