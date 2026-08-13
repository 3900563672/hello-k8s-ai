import { BarChart3, FileSearch, Settings2 } from 'lucide-react'
import { Link, NavLink } from 'react-router-dom'
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'
import { ClusterStatus } from './ClusterStatus'
import { ExecutionControls } from './ExecutionControls'

const navigation = [
    { title: '配置中心', description: '模型、节点与租户', to: '/config', icon: Settings2 },
    { title: '流量布置', description: '模板与租户流量', to: '/traffic', icon: BarChart3 },
    { title: '数据回显', description: '历史指标与结果', to: '/trace', icon: FileSearch },
] as const

export function AppSidebar() {
    return (
        <aside className="relative z-50 flex h-dvh w-[72px] shrink-0 flex-col border-r border-white/[0.07] bg-[#080B11] text-[#DCE5F0] shadow-[14px_0_44px_rgba(0,0,0,.18)]">
            <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_50%_0%,rgba(91,140,255,.09),transparent_24%)]" />

            <div className="relative flex h-16 shrink-0 items-center justify-center border-b border-white/[0.065]">
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Link
                            to="/config"
                            aria-label="调度控制台首页"
                            className="group flex h-10 w-10 items-center justify-center rounded-xl border border-white/[0.08] bg-white/[0.025] outline-none transition duration-150 hover:border-[#5B8CFF]/30 hover:bg-[#5B8CFF]/[0.08] focus-visible:ring-2 focus-visible:ring-[#5B8CFF]/45"
                        >
                            <SchedulerMark />
                        </Link>
                    </TooltipTrigger>
                    <TooltipContent
                        side="right"
                        sideOffset={12}
                        className="border border-white/[0.08] bg-[#111722] text-[#DDE5F0]"
                    >
                        调度控制台
                    </TooltipContent>
                </Tooltip>
            </div>

            <nav
                aria-label="主导航"
                className="relative flex min-h-0 flex-1 flex-col items-center justify-center gap-3 px-2 py-5"
            >
                {navigation.map((item) => (
                    <Tooltip key={item.to}>
                        <TooltipTrigger asChild>
                            <NavLink
                                to={item.to}
                                className={({ isActive }) =>
                                    cn(
                                        'group/nav relative flex h-11 w-11 items-center justify-center rounded-xl border border-transparent text-[#687487] outline-none transition duration-150 hover:border-white/[0.055] hover:bg-white/[0.04] hover:text-[#CFD8E5] focus-visible:ring-2 focus-visible:ring-[#5B8CFF]/40',
                                        isActive &&
                                            'border-[#5B8CFF]/20 bg-[#5B8CFF]/[0.09] text-[#EDF5FF] shadow-[inset_0_0_18px_rgba(91,140,255,.045)]',
                                    )
                                }
                            >
                                {({ isActive }) => (
                                    <>
                                        <span
                                            className={cn(
                                                'absolute -left-[9px] h-5 w-0.5 rounded-r-full bg-[#5B8CFF] opacity-0 shadow-[0_0_10px_rgba(91,140,255,.72)] transition-opacity',
                                                isActive && 'opacity-100',
                                            )}
                                        />
                                        <item.icon className="h-[19px] w-[19px]" strokeWidth={1.8} />
                                        <span className="sr-only">{item.title}</span>
                                    </>
                                )}
                            </NavLink>
                        </TooltipTrigger>
                        <TooltipContent
                            side="right"
                            sideOffset={12}
                            className="border border-white/[0.08] bg-[#111722] px-3 py-2 text-[#DDE5F0]"
                        >
                            <div className="text-[11px] font-medium">{item.title}</div>
                            <div className="mt-0.5 text-[9px] text-[#748196]">{item.description}</div>
                        </TooltipContent>
                    </Tooltip>
                ))}
            </nav>

            <div className="relative shrink-0 border-t border-white/[0.065] px-1.5 py-1.5">
                <ClusterStatus />
            </div>

            <div className="relative h-[82px] shrink-0 border-t border-white/[0.065] px-1">
                <ExecutionControls />
            </div>
        </aside>
    )
}

function SchedulerMark() {
    return (
        <svg
            viewBox="0 0 32 32"
            className="h-6 w-6 overflow-visible transition duration-150 group-hover:scale-105 group-hover:opacity-90"
            aria-hidden="true"
        >
            <path
                d="M16 4.2 25.7 9.8v11.3L16 26.7 6.3 21.1V9.8L16 4.2Z"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.7"
                className="text-[#E9F1FB]"
            />
            <path
                d="m6.8 10 9.2 5.2 9.2-5.2M16 15.2v10.6"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.35"
                className="text-[#789FEA]"
            />
            <circle cx="16" cy="15.2" r="2.25" className="fill-[#5B8CFF]" />
            <circle cx="16" cy="15.2" r="4.2" className="fill-none stroke-[#5B8CFF]/35" />
        </svg>
    )
}
