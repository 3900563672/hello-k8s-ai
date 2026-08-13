import { AlertTriangle, ArrowLeft, LoaderCircle } from 'lucide-react'
import {
    Link,
    isRouteErrorResponse,
    useRouteError,
} from 'react-router-dom'
import { Button } from '@/components/ui/button'

export function PageLoader() {
    return (
        <div
            className="flex h-full min-h-72 items-center justify-center bg-[#05070A]"
            role="status"
            aria-label="页面加载中"
        >
            <div className="flex items-center gap-2.5 rounded-xl border border-white/[0.07] bg-[#0A0E15] px-4 py-3 text-xs text-[#8692A5]">
                <LoaderCircle className="h-4 w-4 animate-spin text-[#73A8FF]" />
                正在载入工作区
            </div>
        </div>
    )
}

export function RouteErrorBoundary() {
    const error = useRouteError()
    const description = isRouteErrorResponse(error)
        ? `${error.status} ${error.statusText}`
        : error instanceof Error
          ? error.message
          : '页面发生了未知错误'

    return (
        <div className="flex h-dvh items-center justify-center bg-[#05070A] p-6 text-[#E8EEF7]">
            <div className="w-full max-w-md rounded-2xl border border-red-400/15 bg-[#0B0E14] p-6 text-center shadow-2xl">
                <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-xl border border-red-400/20 bg-red-400/[0.07] text-red-300">
                    <AlertTriangle className="h-5 w-5" />
                </div>
                <h1 className="mt-4 text-base font-semibold">工作区无法打开</h1>
                <p className="mt-2 text-xs leading-6 text-[#748196]">{description}</p>
                <Button asChild className="mt-5 h-9 bg-[#5B8CFF] text-xs hover:bg-[#70A0FF]">
                    <Link to="/config">返回配置中心</Link>
                </Button>
            </div>
        </div>
    )
}

export function NotFoundPage() {
    return (
        <div className="flex h-full items-center justify-center bg-[#05070A] p-6 text-[#E8EEF7]">
            <div className="text-center">
                <div className="font-mono text-5xl font-semibold tracking-[-0.06em] text-[#253047]">404</div>
                <h1 className="mt-4 text-base font-semibold">没有这个工作区页面</h1>
                <p className="mt-2 text-xs text-[#667286]">地址可能已变更，或功能尚未接入。</p>
                <Button asChild variant="outline" className="mt-5 h-9 border-white/[0.09] bg-white/[0.025] text-xs text-[#CBD5E2] hover:bg-white/[0.06] hover:text-white">
                    <Link to="/config">
                        <ArrowLeft className="mr-2 h-3.5 w-3.5" />
                        返回配置中心
                    </Link>
                </Button>
            </div>
        </div>
    )
}
