import { useRef } from 'react'
import { Loader2, Pencil } from 'lucide-react'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'

interface RenameDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    typeLabel: string
    resourceId: string
    value: string
    onValueChange: (value: string) => void
    pending?: boolean
    error?: string
    onConfirm: () => void
}

export function RenameDialog({
    open,
    onOpenChange,
    typeLabel,
    resourceId,
    value,
    onValueChange,
    pending = false,
    error = '',
    onConfirm,
}: RenameDialogProps) {
    const inputRef = useRef<HTMLInputElement>(null)

    return (
        <Dialog open={open} onOpenChange={(nextOpen) => !pending && onOpenChange(nextOpen)}>
            <DialogContent
                className="max-w-md border-[#263244] bg-[#111] p-0 text-[#F4F4F5] shadow-2xl"
                onOpenAutoFocus={(event) => {
                    event.preventDefault()
                    inputRef.current?.focus()
                    inputRef.current?.select()
                }}
            >
                <DialogHeader className="px-6 pb-2 pt-6 text-left">
                    <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg border border-blue-500/20 bg-blue-500/10">
                        <Pencil className="h-4 w-4 text-[#67A6FF]" />
                    </div>
                    <DialogTitle className="text-base">重命名{typeLabel}</DialogTitle>
                    <DialogDescription className="leading-6 text-[#858585]">
                        只修改界面显示名称，稳定标识不会改变。
                    </DialogDescription>
                </DialogHeader>
                <div className="space-y-3 px-6 py-3">
                    <div>
                        <Label htmlFor="rename-resource-name" className="text-xs font-medium text-[#A0A0A0]">
                            显示名称
                        </Label>
                        <Input
                            id="rename-resource-name"
                            ref={inputRef}
                            value={value}
                            onChange={(event) => onValueChange(event.target.value)}
                            onKeyDown={(event) => {
                                if (event.key === 'Enter' && value.trim() && !pending) {
                                    event.preventDefault()
                                    onConfirm()
                                }
                            }}
                            className="mt-2 border-[#303C50] bg-[#0A0A0A] text-[#F2F2F2] focus-visible:border-[#5B8CFF]/60 focus-visible:ring-2 focus-visible:ring-[#5B8CFF]/15"
                        />
                    </div>
                    <div className="rounded-md border border-[#202B3A] bg-[#0B1018] px-3 py-2">
                        <p className="text-[11px] text-[#596579]">稳定标识</p>
                        <code className="mt-1 block truncate font-mono text-xs text-[#9A9A9A]">{resourceId}</code>
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
                        className="min-w-24 bg-[#F4F4F5] text-[#111] hover:bg-white"
                    >
                        {pending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        保存名称
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
