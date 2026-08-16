import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { format } from 'date-fns'
import type { PreviewConfig } from '@/types/config.types'

interface Template<T = any> {
    id: string
    name: string
    data: T
    createdAt: string
    preset?: boolean
}

interface TemplateLibraryDialogProps<T> {
    open: boolean
    onOpenChange: (open: boolean) => void
    templates: Template<T>[]
    typeLabel: string
    onLoad: (template: Template<T>) => void
    onDelete: (id: string) => void
    // 接收一个函数，把数据转成 PreviewConfig
    getPreview: (data: T) => PreviewConfig
    // pickMode 用于“从模板新建”：隐藏删除按钮，加载按钮改为“使用此模板”
    pickMode?: boolean
}

export function TemplateLibraryDialog<T>({
                                             open,
                                             onOpenChange,
                                             templates,
                                             typeLabel,
                                             onLoad,
                                             onDelete,
                                             getPreview,
                                         pickMode = false,
                                         }: TemplateLibraryDialogProps<T>) {
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-w-3xl max-h-[80vh] bg-[#0D131C] border-[#222222] p-6">
                <DialogHeader>
                    <DialogTitle className="text-[#FAFAFA] text-lg">
                        {pickMode ? '从模板新建 — ' : '模板库 — '}{typeLabel}
                    </DialogTitle>
                </DialogHeader>

                {templates.length === 0 ? (
                    <div className="flex items-center justify-center h-40 text-[#666666] text-sm">
                        暂无已保存的{typeLabel}模板
                    </div>
                ) : (
                    <ScrollArea className="max-h-[55vh] pr-4">
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            {templates.map((template) => {
                                const previewData = getPreview(template.data)
                                return (
                                    <Card
                                        key={template.id}
                                        className="bg-[#0A0A0A] border-[#222222] hover:border-[#444444] transition-colors"
                                    >
                                        <CardHeader className="pb-2">
                                            <div className="flex items-center justify-between gap-2">
                                                <CardTitle className="min-w-0 text-[#FAFAFA] text-sm font-medium truncate">
                                                    {template.name}
                                                </CardTitle>
                                                {template.preset && (
                                                    <Badge
                                                        variant="outline"
                                                        className="h-[18px] shrink-0 border-[#5B8CFF]/25 bg-[#5B8CFF]/10 px-1.5 text-[9px] font-medium text-[#8CB8F8]"
                                                    >
                                                        预置
                                                    </Badge>
                                                )}
                                            </div>
                                            <div className="text-[#666666] text-xs">
                                                {format(new Date(template.createdAt), 'yyyy-MM-dd HH:mm')}
                                            </div>
                                        </CardHeader>
                                        <CardContent className="pb-2">
                                            <div className="text-[#888888] text-xs space-y-1">
                                                {previewData.map((field) => (
                                                    <div key={field.key} className="flex justify-between">
                                                        <span>{field.key}</span>
                                                        <span className="text-[#FAFAFA]">
                                                            {field.value}
                                                            {field.unit && ` ${field.unit}`}
                                                        </span>
                                                    </div>
                                                ))}
                                            </div>
                                        </CardContent>
                                        <CardFooter className="flex gap-2 pt-2">
                                            <Button
                                                size="sm"
                                                className="flex-1 bg-[#5B8CFF] hover:bg-[#0060D0] text-white h-7 text-xs"
                                                onClick={() => onLoad(template)}
                                            >
                                                {pickMode ? '使用此模板' : '加载模板'}
                                            </Button>
                                            {!pickMode && (
                                                <Button
                                                    size="sm"
                                                    variant="ghost"
                                                    className="text-red-400 hover:text-red-300 hover:bg-[#222222] h-7 px-3 text-xs"
                                                    onClick={() => onDelete(template.id)}
                                                >
                                                    删除
                                                </Button>
                                            )}
                                        </CardFooter>
                                    </Card>
                                )
                            })}
                        </div>
                    </ScrollArea>
                )}
            </DialogContent>
        </Dialog>
    )
}
