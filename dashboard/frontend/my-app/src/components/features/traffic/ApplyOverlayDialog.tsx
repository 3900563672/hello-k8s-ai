import { useEffect, useMemo, useState, type ChangeEvent } from 'react'
import { Building2, Clock3, Layers3, MoveRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { useTrafficStore } from '@/stores/trafficSlice'
import type { TenantInfo, TrafficTemplate } from '@/types/traffic.types'
import { PreviewCanvas } from './PreviewCanvas'
import {
    formatLogicalTime,
    formatQps,
    getTemplateDuration,
    getTemplatePeakQps,
    getTemplateValueAtTime,
} from './trafficMath'
import { useReplayTimeContext } from '@/stores/timeSlice'
import { formatUtcTimestamp } from '@/lib/formatters/timeFormatter'

interface ApplyOverlayDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    template: TrafficTemplate | null
    tenants: TenantInfo[]
    pending?: boolean
    onApply: (data: {
        templateId: string
        tenantId: string
        startOffsetSeconds: number
    }) => void
}

export function ApplyOverlayDialog({
    open,
    onOpenChange,
    template,
    tenants,
    pending = false,
    onApply,
}: ApplyOverlayDialogProps) {
    const replay = useReplayTimeContext()
    const overlays = useTrafficStore((state) => state.overlays)
    const [tenantId, setTenantId] = useState('')
    const [offsetText, setOffsetText] = useState('')
    const [attempted, setAttempted] = useState(false)

    useEffect(() => {
        if (!open) return
        setTenantId('')
        setOffsetText('')
        setAttempted(false)
    }, [open, template?.id])

    const offsetSeconds = useMemo(() => {
        if (offsetText.trim() === '') return null
        const value = Number(offsetText)
        return Number.isFinite(value) && value >= 0 ? value : null
    }, [offsetText])

    const selectedTenant = tenants.find((tenant) => tenant.id === tenantId)
    const currentOverlayCount = overlays.filter(
        (overlay) => overlay.tenantId === tenantId && overlay.enabled,
    ).length
    const canSubmit = Boolean(template && selectedTenant && offsetSeconds !== null)
    const historical = replay.mode === 'historical'
    const templateStartQps = template ? getTemplateValueAtTime(template, 0) : 0
    const targetQps = Math.max(
        0,
        Math.round((selectedTenant?.requestedQPS ?? 0) + templateStartQps),
    )

    const handleSubmit = () => {
        setAttempted(true)
        if (!template || !selectedTenant || offsetSeconds === null) return
        onApply({
            templateId: template.id,
            tenantId: selectedTenant.id,
            startOffsetSeconds: offsetSeconds,
        })
        onOpenChange(false)
    }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-h-[92vh] max-w-5xl overflow-y-auto border-white/[0.08] bg-[#0A0E15] p-0 text-[#E8EEF7] shadow-2xl">
                <DialogHeader className="border-b border-white/[0.06] px-6 pb-4 pt-5">
                    <div className="flex items-center gap-3">
                        <div className="flex h-10 w-10 items-center justify-center rounded-xl border border-[#5B8CFF]/25 bg-[#5B8CFF]/10 text-[#77ADFF]">
                            <Layers3 className="h-4 w-4" />
                        </div>
                        <div>
                            <DialogTitle className="text-base font-semibold text-[#F0F5FB]">配置流量叠加</DialogTitle>
                            <p className="mt-1 text-[11px] text-[#657184]">{template?.name ?? '未选择模板'} · 纯 QPS 加法</p>
                        </div>
                    </div>
                </DialogHeader>

                <div className="grid grid-cols-1 gap-0 lg:grid-cols-[340px_minmax(0,1fr)]">
                    <div className="border-b border-white/[0.06] px-6 py-5 lg:border-b-0 lg:border-r">
                        {template && (
                            <div className="mb-5 grid grid-cols-2 gap-2">
                                <Metric label="模板时长" value={formatLogicalTime(getTemplateDuration(template))} />
                                <Metric label="峰值增量" value={`${formatQps(getTemplatePeakQps(template))} QPS`} />
                            </div>
                        )}

                        <div className="mb-5 rounded-xl border border-[#5B8CFF]/15 bg-[#5B8CFF]/[0.045] p-3">
                            <div className="flex items-center justify-between gap-3 text-[9px] text-[#61708A]">
                                <span>场景基准切面</span>
                                <span className={replay.mode === 'historical' ? 'text-[#A8B5FF]' : 'text-emerald-300'}>
                                    {replay.mode === 'historical' ? '历史' : '最新'}
                                </span>
                            </div>
                            <div className="mt-1.5 truncate font-mono text-[10px] text-[#AAB8CC]">
                                {formatUtcTimestamp(replay.effectiveAt, true)} UTC
                            </div>
                            <div className="mt-1 truncate font-mono text-[8px] text-[#4F5B70]">
                                {replay.snapshotId ?? 'snapshot-unavailable'}
                            </div>
                        </div>

                        <div className="space-y-5">
                            <div className="space-y-2">
                                <Label className="flex items-center gap-1.5 text-[11px] font-medium text-[#8C98AA]">
                                    <Building2 className="h-3.5 w-3.5" />
                                    目标租户
                                </Label>
                                <Select value={tenantId} onValueChange={setTenantId}>
                                    <SelectTrigger className="h-10 border-white/[0.08] bg-white/[0.025] text-xs text-[#E6EDF6] focus:ring-[#5B8CFF]/25">
                                        <SelectValue placeholder="请选择租户（不预选）" />
                                    </SelectTrigger>
                                    <SelectContent className="border-white/[0.08] bg-[#0E131C] text-[#E6EDF6]">
                                        {tenants.map((tenant) => (
                                            <SelectItem key={tenant.id} value={tenant.id}>
                                                <span className="flex items-center gap-2">
                                                    <span>{tenant.name}</span>
                                                    <span className="text-[9px] text-[#667286]">{tenant.priority}</span>
                                                </span>
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                {attempted && !tenantId && (
                                    <p className="text-[10px] text-amber-300">请选择模板实际归属的租户</p>
                                )}
                            </div>

                            <div className="space-y-2">
                                <Label htmlFor="traffic-offset" className="flex items-center gap-1.5 text-[11px] font-medium text-[#8C98AA]">
                                    <Clock3 className="h-3.5 w-3.5" />
                                    逻辑开始偏移（秒）
                                </Label>
                                <div className="relative">
                                    <Input
                                        id="traffic-offset"
                                        type="number"
                                        min={0}
                                        step="any"
                                        inputMode="decimal"
                                        value={offsetText}
                                        onChange={(event: ChangeEvent<HTMLInputElement>) => setOffsetText(event.target.value)}
                                        placeholder="例如 30"
                                        className="h-10 border-white/[0.08] bg-white/[0.025] pr-12 text-xs text-[#E6EDF6] placeholder:text-[#505B6C] focus-visible:border-[#5B8CFF]/55 focus-visible:ring-[#5B8CFF]/15"
                                    />
                                    <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-[10px] text-[#5F6B7E]">秒</span>
                                </div>
                                {attempted && offsetSeconds === null && (
                                    <p className="text-[10px] text-amber-300">请输入大于或等于 0 的逻辑时间</p>
                                )}
                                <p className="text-[10px] leading-4 text-[#566174]">T+0 表示场景起点；模板持续时间由绘制坐标自动决定。</p>
                            </div>
                        </div>

                        <div className="mt-6 rounded-xl border border-white/[0.06] bg-white/[0.018] p-3">
                            <div className="flex items-center gap-2 text-[10px] text-[#697588]">
                                <span className="rounded-md bg-[#5B8CFF]/10 px-1.5 py-1 text-[#7BAFFF]">当前</span>
                                <span>{selectedTenant ? `${selectedTenant.name} 已启用 ${currentOverlayCount} 个叠加` : '等待选择租户'}</span>
                            </div>
                            <div className="mt-2 flex items-center gap-2 text-[10px] text-[#7D899B]">
                                <span>原流量</span>
                                <MoveRight className="h-3 w-3" />
                                <span className="text-[#8BBCFF]">原流量 + 模板增量</span>
                            </div>
                            {selectedTenant && (
                                <div className="mt-3 border-t border-white/[0.06] pt-3 text-[10px] leading-4 text-[#7D899B]">
                                    <div className="flex items-center justify-between">
                                        <span>应用后租户目标 QPS</span>
                                        <span className="font-mono text-[#8BBCFF]">{formatQps(targetQps)}</span>
                                    </div>
                                    <p className="mt-1.5 text-[9px] text-[#55617A]">
                                        控制面为常量目标 QPS；叠加曲线仅作场景预览，实际按起始增量写入
                                    </p>
                                </div>
                            )}
                        </div>
                    </div>

                    <div className="min-h-[350px] px-5 py-5 lg:min-h-[430px]">
                        <div className="mb-3 flex items-center justify-between">
                            <div>
                                <div className="text-xs font-medium text-[#CFD8E5]">实时预览</div>
                                <div className="mt-1 text-[10px] text-[#5F6B7E]">横轴始终使用从 0 开始的逻辑秒</div>
                            </div>
                            {offsetSeconds !== null && (
                                <span className="rounded-full border border-[#5B8CFF]/20 bg-[#5B8CFF]/8 px-2 py-1 font-mono text-[9px] text-[#7FB4FF]">
                                    T+{formatLogicalTime(offsetSeconds)}
                                </span>
                            )}
                        </div>
                        <div className="h-[330px]">
                            <PreviewCanvas
                                template={template}
                                tenantId={tenantId}
                                offsetSeconds={offsetSeconds}
                            />
                        </div>
                    </div>
                </div>

                <DialogFooter className="border-t border-white/[0.06] px-6 py-4">
                    {historical && (
                        <p className="mr-auto text-[10px] text-amber-300">
                            历史模式只读，不能应用流量
                        </p>
                    )}
                    <Button
                        variant="ghost"
                        onClick={() => onOpenChange(false)}
                        className="text-[#8490A2] hover:bg-white/[0.05] hover:text-white"
                    >
                        取消
                    </Button>
                    <Button
                        onClick={handleSubmit}
                        disabled={pending || !canSubmit || historical}
                        aria-disabled={pending || !canSubmit || historical}
                        className="bg-[#5B8CFF] px-5 text-white hover:bg-[#70A0FF] aria-disabled:opacity-50"
                    >
                        确认叠加
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

function Metric({ label, value }: { label: string; value: string }) {
    return (
        <div className="rounded-xl border border-white/[0.06] bg-white/[0.02] px-3 py-2.5">
            <div className="text-[9px] text-[#5F6B7E]">{label}</div>
            <div className="mt-1 font-mono text-[11px] font-medium text-[#C9D5E3]">{value}</div>
        </div>
    )
}
