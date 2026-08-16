import { type MouseEvent, type PointerEvent, useMemo, useState } from 'react'
import { useDraggable } from '@dnd-kit/core'
import {
    BookOpen,
    Clock3,
    Eye,
    GripVertical,
    PencilLine,
    Plus,
    Sparkles,
    Trash2,
} from 'lucide-react'
import { Link } from 'react-router-dom'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useTrafficStore } from '@/stores/trafficSlice'
import { cn } from '@/lib/utils'
import type { TrafficPoint, TrafficTemplate } from '@/types/traffic.types'
import { PreviewCurve } from './PreviewCurve'
import {
    formatLogicalTime,
    formatQps,
    getTemplateDuration,
    getTemplatePeakQps,
    sanitizeControlPoints,
} from './trafficMath'

interface TemplateLibraryProps {
    onApply: (template: TrafficTemplate) => void
}

interface TemplateCardProps {
    template: TrafficTemplate
    onApply: (template: TrafficTemplate) => void
    onDelete: (id: string) => void
    onView: (template: TrafficTemplate) => void
}

function getMiniCurve(pointsInput: TrafficPoint[]) {
    const points = sanitizeControlPoints(pointsInput)
    if (points.length < 2) return { line: '', area: '' }
    const width = 216
    const height = 58
    const paddingX = 7
    const paddingY = 7
    const maxX = Math.max(1, points.at(-1)?.x ?? 1)
    const maxY = Math.max(1, ...points.map((point) => point.y))
    const screenPoints = points.map((point) => ({
        x: paddingX + (point.x / maxX) * (width - paddingX * 2),
        y: height - paddingY - (point.y / maxY) * (height - paddingY * 2),
    }))
    const line = screenPoints
        .map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x.toFixed(2)} ${point.y.toFixed(2)}`)
        .join(' ')
    const first = screenPoints[0]
    const last = screenPoints[screenPoints.length - 1]
    return {
        line,
        area: `${line} L ${last.x.toFixed(2)} ${height - paddingY} L ${first.x.toFixed(2)} ${height - paddingY} Z`,
    }
}

function TemplateCard({ template, onApply, onDelete, onView }: TemplateCardProps) {
    const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
        id: template.id,
        data: { type: 'traffic-template', template },
    })
    const duration = getTemplateDuration(template)
    const peak = getTemplatePeakQps(template)
    const curve = useMemo(() => getMiniCurve(template.controlPoints), [template.controlPoints])
    const style = transform
        ? { transform: `translate3d(${transform.x}px, ${transform.y}px, 0)`, zIndex: 50 }
        : undefined

    const stopDrag = (event: PointerEvent<HTMLButtonElement>) => event.stopPropagation()

    return (
        <Card
            ref={setNodeRef}
            style={style}
            {...attributes}
            {...listeners}
            onClick={() => !isDragging && onApply(template)}
            className={cn(
                'group touch-none select-none overflow-hidden rounded-xl border-white/[0.07] bg-[#0B1018] shadow-none transition-all duration-200 hover:-translate-y-0.5 hover:border-[#5B8CFF]/30 hover:bg-[#0E141E] hover:shadow-[0_16px_36px_rgba(0,0,0,.22)]',
                isDragging ? 'cursor-grabbing opacity-45' : 'cursor-grab opacity-100',
            )}
        >
            <CardContent className="p-3">
                <div className="mb-2.5 flex items-center gap-2">
                    <GripVertical className="h-3.5 w-3.5 shrink-0 text-[#3D4859] transition-colors group-hover:text-[#657287]" />
                    <div className="min-w-0 flex-1 truncate text-xs font-medium text-[#E4EBF5]">
                        {template.name}
                    </div>
                    <Button
                        variant="ghost"
                        size="icon"
                        title="删除模板"
                        className="h-6 w-6 shrink-0 rounded-md text-[#566276] opacity-0 hover:bg-red-500/10 hover:text-red-300 group-hover:opacity-100"
                        onPointerDown={stopDrag}
                        onClick={(event: MouseEvent<HTMLButtonElement>) => {
                            event.stopPropagation()
                            onDelete(template.id)
                        }}
                    >
                        <Trash2 className="h-3 w-3" />
                    </Button>
                </div>

                <div className="relative h-[58px] overflow-hidden rounded-lg border border-white/[0.05] bg-[#070A10]">
                    <div className="absolute inset-0 bg-[linear-gradient(rgba(148,163,184,.035)_1px,transparent_1px),linear-gradient(90deg,rgba(148,163,184,.035)_1px,transparent_1px)] bg-[size:24px_18px]" />
                    <svg viewBox="0 0 216 58" className="relative h-full w-full" preserveAspectRatio="none" aria-hidden="true">
                        <defs>
                            <linearGradient id={`mini-line-${template.id}`} x1="0" x2="1">
                                <stop offset="0" stopColor="#5B8CFF" />
                                <stop offset="1" stopColor="#8CC8FF" />
                            </linearGradient>
                            <linearGradient id={`mini-area-${template.id}`} x1="0" y1="0" x2="0" y2="1">
                                <stop offset="0" stopColor="#5B8CFF" stopOpacity="0.25" />
                                <stop offset="1" stopColor="#5B8CFF" stopOpacity="0" />
                            </linearGradient>
                        </defs>
                        {curve.area && <path d={curve.area} fill={`url(#mini-area-${template.id})`} />}
                        {curve.line && (
                            <path
                                d={curve.line}
                                fill="none"
                                stroke={`url(#mini-line-${template.id})`}
                                strokeWidth="2"
                                strokeLinecap="round"
                                strokeLinejoin="round"
                            />
                        )}
                    </svg>
                </div>

                <div className="mt-2.5 flex items-center gap-1.5">
                    <Badge variant="outline" className="h-5 gap-1 border-white/[0.07] bg-white/[0.025] px-1.5 text-[9px] font-normal text-[#778397]">
                        <Clock3 className="h-2.5 w-2.5" />
                        {formatLogicalTime(duration)}
                    </Badge>
                    <Badge variant="outline" className="h-5 border-white/[0.07] bg-white/[0.025] px-1.5 text-[9px] font-normal text-[#778397]">
                        峰值 {formatQps(peak)}
                    </Badge>
                    <Button
                        variant="ghost"
                        size="sm"
                        className="ml-auto h-5 px-1.5 text-[9px] text-[#647186] hover:bg-white/[0.05] hover:text-white"
                        onPointerDown={stopDrag}
                        onClick={(event: MouseEvent<HTMLButtonElement>) => {
                            event.stopPropagation()
                            onView(template)
                        }}
                    >
                        <Eye className="mr-1 h-3 w-3" />
                        详情
                    </Button>
                </div>
            </CardContent>
        </Card>
    )
}

export function TemplateLibrary({ onApply }: TemplateLibraryProps) {
    const { templates, removeTemplate, setMode } = useTrafficStore()
    const [detailTemplate, setDetailTemplate] = useState<TrafficTemplate | null>(null)

    return (
        <div className="flex h-full min-h-0 flex-col">
            <div className="shrink-0 border-b border-white/[0.06] px-4 pb-4 pt-4">
                <div className="mb-3 flex items-start justify-between gap-3">
                    <div>
                        <div className="flex items-center gap-2">
                            <h2 className="text-xs font-semibold text-[#E1E8F2]">流量模板</h2>
                            <span className="rounded-full border border-white/[0.06] bg-white/[0.025] px-1.5 py-0.5 text-[9px] text-[#6F7C90]">
                                {templates.length}
                            </span>
                        </div>
                        <p className="mt-1 text-[10px] leading-4 text-[#5E6A7D]">点击配置 · 拖到画布 · 查看真实坐标</p>
                    </div>
                    <Sparkles className="mt-0.5 h-4 w-4 text-[#5B8CFF]/70" />
                </div>
                <Button
                    size="sm"
                    onClick={() => setMode('draw')}
                    className="h-9 w-full rounded-lg bg-[#5B8CFF] text-[11px] font-medium text-white shadow-[0_10px_26px_rgba(91,140,255,.18)] hover:bg-[#70A0FF]"
                >
                    <Plus className="mr-1.5 h-3.5 w-3.5" />
                    绘制新模板
                </Button>
            </div>

            <ScrollArea className="min-h-0 flex-1">
                <div className="space-y-2.5 p-3">
                    {templates.length === 0 ? (
                        <div className="rounded-xl border border-dashed border-white/[0.08] bg-white/[0.012] px-5 py-10 text-center">
                            <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-xl border border-white/[0.07] bg-white/[0.025] text-[#657287]">
                                <PencilLine className="h-4 w-4" />
                            </div>
                            <div className="text-xs font-medium text-[#8995A8]">模板库为空</div>
                            <p className="mt-1.5 text-[10px] leading-4 text-[#566174]">从真实的秒 / QPS 坐标开始绘制第一条流量曲线</p>
                            <Link
                                to="/guide"
                                className="mt-3 inline-flex h-7 items-center gap-1.5 rounded-md border border-white/[0.08] bg-white/[0.025] px-3 text-[10px] text-[#8995A8] transition-colors hover:bg-white/[0.06] hover:text-white"
                            >
                                <BookOpen className="h-3.5 w-3.5" />
                                查看填写指南
                            </Link>
                        </div>
                    ) : templates.map((template) => (
                        <TemplateCard
                            key={template.id}
                            template={template}
                            onApply={onApply}
                            onDelete={removeTemplate}
                            onView={setDetailTemplate}
                        />
                    ))}
                </div>
            </ScrollArea>

            <Dialog
                open={detailTemplate !== null}
                onOpenChange={(open: boolean) => !open && setDetailTemplate(null)}
            >
                <DialogContent className="max-w-3xl border-white/[0.08] bg-[#0B0F16] p-0 text-[#E8EEF7] shadow-2xl">
                    {detailTemplate && (
                        <>
                            <DialogHeader className="border-b border-white/[0.06] px-6 pb-4 pt-5">
                                <DialogTitle className="text-base font-semibold text-[#EDF3FA]">
                                    {detailTemplate.name}
                                </DialogTitle>
                                <p className="mt-1 text-[11px] text-[#657184]">模板原始坐标 · 未归一化</p>
                            </DialogHeader>
                            <div className="px-6 py-5">
                                <div className="h-[300px] overflow-hidden rounded-xl border border-white/[0.06] bg-[#070B11]">
                                    <PreviewCurve template={detailTemplate} />
                                </div>
                                <div className="mt-4 grid grid-cols-3 gap-3">
                                    <Stat label="持续时间" value={formatLogicalTime(getTemplateDuration(detailTemplate))} />
                                    <Stat label="峰值 QPS" value={formatQps(getTemplatePeakQps(detailTemplate))} />
                                    <Stat label="控制点" value={`${detailTemplate.controlPoints.length}`} />
                                </div>
                            </div>
                            <DialogFooter className="border-t border-white/[0.06] px-6 py-4">
                                <Button
                                    variant="ghost"
                                    onClick={() => setDetailTemplate(null)}
                                    className="text-[#8490A2] hover:bg-white/[0.05] hover:text-white"
                                >
                                    关闭
                                </Button>
                                <Button
                                    onClick={() => {
                                        onApply(detailTemplate)
                                        setDetailTemplate(null)
                                    }}
                                    className="bg-[#5B8CFF] text-white hover:bg-[#70A0FF]"
                                >
                                    叠加此模板
                                </Button>
                            </DialogFooter>
                        </>
                    )}
                </DialogContent>
            </Dialog>
        </div>
    )
}

function Stat({ label, value }: { label: string; value: string }) {
    return (
        <div className="rounded-xl border border-white/[0.06] bg-white/[0.02] px-4 py-3">
            <div className="text-[10px] text-[#667286]">{label}</div>
            <div className="mt-1 font-mono text-sm font-medium text-[#DCE6F2]">{value}</div>
        </div>
    )
}
