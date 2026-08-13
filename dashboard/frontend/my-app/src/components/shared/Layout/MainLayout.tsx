import { Outlet } from 'react-router-dom'
import { useWorkspaceCoordinator } from '@/hooks/useWorkspaceContext'
import { useBackendSync } from '@/hooks/useBackendSync'
import { AppSidebar } from './AppSidebar'
import { TimeTravelBar } from '../TimeTravelBar/TimeTravelBar'

/**
 * 稳定的应用壳层。页面只负责内部内容，导航、时间与执行语义始终常驻。
 */
export function MainLayout() {
    useWorkspaceCoordinator()
    useBackendSync()

    return (
        <div className="flex h-dvh min-h-0 w-full overflow-hidden bg-[#05070A] text-[#E8EEF7]">
            <a
                href="#main-content"
                className="fixed left-20 top-2 z-[120] -translate-y-16 rounded-lg bg-[#5B8CFF] px-3 py-2 text-xs text-white outline-none transition-transform focus:translate-y-0"
            >
                跳到主内容
            </a>
            <AppSidebar />
            <div className="relative flex min-w-0 flex-1 flex-col overflow-hidden">
                <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_72%_-15%,rgba(91,140,255,.055),transparent_28%)]" />
                <TimeTravelBar />
                <main
                    id="main-content"
                    tabIndex={-1}
                    className="relative min-h-0 flex-1 overflow-hidden outline-none"
                >
                    <Outlet />
                </main>
            </div>
        </div>
    )
}
