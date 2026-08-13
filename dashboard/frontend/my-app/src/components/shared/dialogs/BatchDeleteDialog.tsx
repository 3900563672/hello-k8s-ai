import { AlertTriangle, Loader2 } from 'lucide-react'
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from '@/components/ui/alert-dialog'

interface BatchDeleteDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    typeLabel: string
    count: number
    pending?: boolean
    error?: string
    onConfirm: () => void
}

export function BatchDeleteDialog({
    open,
    onOpenChange,
    typeLabel,
    count,
    pending = false,
    error = '',
    onConfirm,
}: BatchDeleteDialogProps) {
    return (
        <AlertDialog open={open} onOpenChange={(nextOpen) => !pending && onOpenChange(nextOpen)}>
            <AlertDialogContent className="max-w-md border-[#263244] bg-[#111] p-0 text-[#F4F4F5] shadow-2xl">
                <AlertDialogHeader className="px-6 pb-2 pt-6 text-left">
                    <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-full border border-red-500/20 bg-red-500/10">
                        <AlertTriangle className="h-5 w-5 text-[#FF6B6B]" />
                    </div>
                    <AlertDialogTitle className="text-base font-semibold">
                        删除 {count} 个{typeLabel}？
                    </AlertDialogTitle>
                    <AlertDialogDescription className="leading-6 text-[#858585]">
                        所选资源会从本地配置中一次性移除，此操作不可撤销。
                    </AlertDialogDescription>
                    {error && (
                        <div className="mt-3 rounded-md border border-red-500/20 bg-red-500/10 px-3 py-2 text-sm text-[#FF8A8A]">
                            {error}
                        </div>
                    )}
                </AlertDialogHeader>
                <AlertDialogFooter className="border-t border-[#202B3A] bg-[#080C12] px-6 py-4">
                    <AlertDialogCancel
                        disabled={pending}
                        className="border-[#303C50] bg-[#141C28] text-[#D4D4D4] hover:bg-[#222] hover:text-white"
                    >
                        取消
                    </AlertDialogCancel>
                    <AlertDialogAction
                        onClick={(event) => {
                            event.preventDefault()
                            onConfirm()
                        }}
                        disabled={pending || count === 0}
                        className="min-w-24 bg-[#E5484D] font-medium text-white hover:bg-[#F2555A]"
                    >
                        {pending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        {pending ? '正在删除' : '确认删除'}
                    </AlertDialogAction>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    )
}
