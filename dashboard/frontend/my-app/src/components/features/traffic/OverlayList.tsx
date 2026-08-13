import { useState, type MouseEvent } from 'react'
import { Clock3, Eye, EyeOff, Layers3, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { useTrafficStore } from '@/stores/trafficSlice'
import { cn } from '@/lib/utils'
import type { OverlayInstance } from '@/types/traffic.types'
import { PreviewCurve } from './PreviewCurve'
import { formatLogicalTime, getTemplateDuration } from './trafficMath'
import { formatUtcTimestamp } from '@/lib/formatters/timeFormatter'

export function OverlayList() {
    const { overlays, templates, removeOverlay, toggleOverlay } = useTrafficStore()
    const [detailOverlay, setDetailOverlay] = useState<OverlayInstance | null>(null)
    const detailTemplate = detailOverlay
        ? templates.find((template) => template.id === detailOverlay.templateId) ?? null
        : null

    return (
        <>
            <div className="flex h-full min-h-0 flex-col bg-[#090D14]">
                <div className="flex h-9 shrink-0 items-center justify-between border-b border-white/[0.05] px-4">
                    <div className="flex items-center gap-2">
                        <Layers3 className="h-3.5 w-3.5 text-[#647186]" />
                        <span className="text-[10px] font-medium text-[#8B97A9]">已布置叠加</span>
                        <span className="rounded-full bg-white/[0.035] px-1.5 py-0.5 text-[9px] text-[#637085]">{overlays.length}</span>
                    </div>
                    <span className="text-[9px] text-[#4F5B6D]">每个叠加只属于一个租户</span>
                </div>

                {overlays.length === 0 ? (
                    <div className="flex min-h-0 flex-1 items-center justify-center gap-2 text-[10px] text-[#536073]">
                        <span className="h-1.5 w-1.5 rounded-full bg-[#5B8CFF]/55" />
                        暂无叠加，点击模板即可配置
                    </div>
                ) : (
                    <div className="min-h-0 flex-1 overflow-x-auto overflow-y-hidden px-3 py-2">
                        <div className="flex h-full min-w-max items-center gap-2">
                            {overlays.map((overlay) => {
                                const template = templates.find((item) => item.id === overlay.templateId)
                                return (
                                    <button
                                        key={overlay.id}
                                        type="button"
                                        onClick={() => setDetailOverlay(overlay)}
                                        className={cn(
                                            'group flex h-[54px] min-w-[230px] items-center gap-3 rounded-xl border border-white/[0.07] bg-[#0C111A] px-3 text-left transition-all hover:border-white/[0.12] hover:bg-[#101722]',
                                            !overlay.enabled && 'opacity-45',
                                        )}
                                    >
                                        <span className="h-8 w-1 shrink-0 rounded-full" style={{ backgroundColor: overlay.color }} />
                                        <span className="min-w-0 flex-1">
                                            <span className="block truncate text-[11px] font-medium text-[#DCE5F0]">{overlay.templateName}</span>
                                            <span className="mt-1 flex items-center gap-1.5 text-[9px] text-[#657184]">
                                                <span className="max-w-[98px] truncate">{overlay.tenantName}</span>
                                                <span>·</span>
                                                <Clock3 className="h-2.5 w-2.5" />
                                                T+{formatLogicalTime(overlay.startOffsetSeconds)}
                                                {template && <span>· {formatLogicalTime(getTemplateDuration(template))}</span>}
                                            </span>
                                        </span>
                                        <span className="flex items-center gap-0.5">
                                            <Button
                                                type="button"
                                                variant="ghost"
                                                size="icon"
                                                title={overlay.enabled ? '禁用叠加' : '启用叠加'}
                                                className="h-7 w-7 text-[#637085] hover:bg-white/[0.06] hover:text-white"
                                                onClick={(event: MouseEvent<HTMLButtonElement>) => {
                                                    event.stopPropagation()
                                                    toggleOverlay(overlay.id)
                                                }}
                                            >
                                                {overlay.enabled ? <Eye className="h-3.5 w-3.5" /> : <EyeOff className="h-3.5 w-3.5" />}
                                            </Button>
                                            <Button
                                                type="button"
                                                variant="ghost"
                                                size="icon"
                                                title="删除叠加"
                                                className="h-7 w-7 text-[#637085] hover:bg-red-500/10 hover:text-red-300"
                                                onClick={(event: MouseEvent<HTMLButtonElement>) => {
                                                    event.stopPropagation()
                                                    removeOverlay(overlay.id)
                                                }}
                                            >
                                                <Trash2 className="h-3.5 w-3.5" />
                                            </Button>
                                        </span>
                                    </button>
                                )
                            })}
                        </div>
                    </div>
                )}
            </div>

            <Dialog
                open={detailOverlay !== null}
                onOpenChange={(open: boolean) => !open && setDetailOverlay(null)}
            >
                <DialogContent className="max-w-3xl border-white/[0.08] bg-[#0B0F16] p-0 text-[#E8EEF7]">
                    {detailOverlay && (
                        <>
                            <DialogHeader className="border-b border-white/[0.06] px-6 pb-4 pt-5">
                                <DialogTitle className="text-base text-[#EDF3FA]">{detailOverlay.templateName}</DialogTitle>
                                <p className="mt-1 text-[11px] text-[#657184]">叠加实例详情</p>
                            </DialogHeader>
                            <div className="px-6 py-5">
                                <div className="mb-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
                                    <Info label="目标租户" value={detailOverlay.tenantName} />
                                    <Info label="逻辑偏移" value={`T+${formatLogicalTime(detailOverlay.startOffsetSeconds)}`} />
                                    <Info label="状态" value={detailOverlay.enabled ? '已启用' : '已禁用'} />
                                    <Info label="基准切面（UTC）" value={formatUtcTimestamp(detailOverlay.effectiveAt)} />
                                </div>
                                <div className="mb-4 truncate rounded-lg border border-[#5B8CFF]/12 bg-[#5B8CFF]/[0.035] px-3 py-2 font-mono text-[9px] text-[#68799A]">
                                    snapshot: {detailOverlay.snapshotId ?? 'legacy / unavailable'}
                                </div>
                                <div className="h-[270px] overflow-hidden rounded-xl border border-white/[0.06] bg-[#070B11]">
                                    {detailTemplate ? (
                                        <PreviewCurve template={detailTemplate} />
                                    ) : (
                                        <div className="flex h-full items-center justify-center text-xs text-[#657184]">原模板已不存在</div>
                                    )}
                                </div>
                            </div>
                            <DialogFooter className="border-t border-white/[0.06] px-6 py-4">
                                <Button
                                    variant="ghost"
                                    onClick={() => setDetailOverlay(null)}
                                    className="text-[#8490A2] hover:bg-white/[0.05] hover:text-white"
                                >
                                    关闭
                                </Button>
                                <Button
                                    variant="outline"
                                    onClick={() => toggleOverlay(detailOverlay.id)}
                                    className="border-white/[0.10] bg-white/[0.025] text-[#C8D2E0] hover:bg-white/[0.06] hover:text-white"
                                >
                                    {detailOverlay.enabled ? <EyeOff className="mr-1.5 h-3.5 w-3.5" /> : <Eye className="mr-1.5 h-3.5 w-3.5" />}
                                    {detailOverlay.enabled ? '禁用' : '启用'}
                                </Button>
                                <Button
                                    variant="ghost"
                                    onClick={() => {
                                        removeOverlay(detailOverlay.id)
                                        setDetailOverlay(null)
                                    }}
                                    className="text-red-300 hover:bg-red-500/10 hover:text-red-200"
                                >
                                    <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                                    删除叠加
                                </Button>
                            </DialogFooter>
                        </>
                    )}
                </DialogContent>
            </Dialog>
        </>
    )
}

function Info({ label, value }: { label: string; value: string }) {
    return (
        <div className="rounded-xl border border-white/[0.06] bg-white/[0.02] px-3 py-2.5">
            <div className="text-[9px] text-[#5F6B7E]">{label}</div>
            <div className="mt-1 truncate text-[11px] font-medium text-[#CBD6E4]">{value}</div>
        </div>
    )
}
