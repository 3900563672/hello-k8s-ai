import { useRef } from 'react'
import { BrainCircuit, Loader2, Plus, Server, SlidersHorizontal, Users } from 'lucide-react'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import type { ConfigResourceType } from '@/types/config.types'

interface CreateDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    type: ConfigResourceType
    value: string
    onValueChange: (value: string) => void
    identifierPreview: string
    nameLabel?: string
    pending?: boolean
    error?: string
    onConfirm: () => void
}

const typeMeta = {
    model: { label: '模型', icon: BrainCircuit, description: '定义推理模型及其性能参数。' },
    node: { label: '节点', icon: Server, description: '定义可参与调度的计算资源。' },
    tenant: { label: '租户', icon: Users, description: '定义业务租户及其调度策略。' },
    orchestrator: {
        label: '编排策略',
        icon: SlidersHorizontal,
        description: '定义租户的扩缩容冷却、副本范围与缩容策略。',
    },
} as const

export function CreateDialog({
    open,
    onOpenChange,
    type,
    value,
    onValueChange,
    identifierPreview,
    nameLabel = '显示名称',
    pending = false,
    error = '',
    onConfirm,
}: CreateDialogProps) {
    const inputRef = useRef<HTMLInputElement>(null)
    const meta = typeMeta[type]
    const Icon = meta.icon

    return (
        <Dialog open={open} onOpenChange={(nextOpen) => !pending && onOpenChange(nextOpen)}>
            <DialogContent
                className="max-w-md border-[#263244] bg-[#111] p-0 text-[#F4F4F5] shadow-2xl"
                onOpenAutoFocus={(event) => {
                    event.preventDefault()
                    inputRef.current?.focus()
                }}
            >
                <DialogHeader className="px-6 pb-2 pt-6 text-left">
                    <div className="mb-2 flex h-10 w-10 items-center justify-center rounded-xl border border-blue-500/20 bg-blue-500/10">
                        <Icon className="h-4.5 w-4.5 text-[#67A6FF]" />
                    </div>
                    <DialogTitle className="text-base font-semibold">新建{meta.label}</DialogTitle>
                    <DialogDescription className="leading-6 text-[#858585]">{meta.description}</DialogDescription>
                </DialogHeader>

                <div className="space-y-3 px-6 py-3">
                    <div>
                        <Label htmlFor="create-resource-name" className="text-xs font-medium text-[#A0A0A0]">
                            {nameLabel}
                        </Label>
                        <Input
                            id="create-resource-name"
                            ref={inputRef}
                            value={value}
                            onChange={(event) => onValueChange(event.target.value)}
                            onKeyDown={(event) => {
                                if (event.key === 'Enter' && value.trim() && !pending) {
                                    event.preventDefault()
                                    onConfirm()
                                }
                            }}
                            placeholder={`输入${nameLabel}`}
                            autoComplete="off"
                            className="mt-2 border-[#303C50] bg-[#0A0A0A] text-[#F2F2F2] placeholder:text-[#555] focus-visible:border-[#5B8CFF]/60 focus-visible:ring-2 focus-visible:ring-[#5B8CFF]/15"
                        />
                    </div>
                    <div className="rounded-md border border-[#202B3A] bg-[#0B1018] px-3 py-2">
                        <p className="text-[11px] text-[#596579]">系统标识</p>
                        <code className="mt-1 block truncate font-mono text-xs text-[#9A9A9A]">
                            {identifierPreview || '输入名称后自动生成'}
                        </code>
                    </div>
                    {error && (
                        <div className="rounded-md border border-red-500/20 bg-red-500/10 px-3 py-2 text-sm text-[#FF8A8A]">
                            {error}
                        </div>
                    )}
                </div>

                <DialogFooter className="border-t border-[#202B3A] bg-[#080C12] px-6 py-4">
                    <Button
                        type="button"
                        variant="outline"
                        disabled={pending}
                        onClick={() => onOpenChange(false)}
                        className="border-[#303C50] bg-[#141C28] text-[#D4D4D4] hover:bg-[#222] hover:text-white"
                    >
                        取消
                    </Button>
                    <Button
                        type="button"
                        disabled={!value.trim() || pending}
                        onClick={onConfirm}
                        className="min-w-24 bg-[#5B8CFF] text-white hover:bg-[#70A0FF]"
                    >
                        {pending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Plus className="mr-2 h-4 w-4" />}
                        {pending ? '正在创建' : '创建资源'}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
