import { useState } from 'react'
import { ChevronRight } from 'lucide-react'

export function CollapsibleSection({ title, subtitle, defaultOpen = false, children }: {
    title: string
    subtitle: string
    defaultOpen?: boolean
    children: React.ReactNode
}) {
    const [open, setOpen] = useState(defaultOpen)
    return (
        <section className="overflow-hidden rounded-xl border border-white/[0.06] bg-[#0A0E15]/60">
            <button
                type="button"
                onClick={() => setOpen((current) => !current)}
                className="flex w-full items-center gap-2.5 px-4 py-3 text-left transition-colors hover:bg-white/[0.02]"
            >
                <ChevronRight className={`h-3.5 w-3.5 text-[#6EA3F8] transition-transform ${open ? 'rotate-90' : ''}`} />
                <span className="text-[12px] font-semibold text-[#CFDAE8]">{title}</span>
                <span className="hidden truncate text-[12px] text-[#536177] sm:block">{subtitle}</span>
                <span className="ml-auto shrink-0 text-[12px] text-[#536177]">{open ? '收起' : '展开'}</span>
            </button>
            {open && <div className="border-t border-white/[0.05] p-4">{children}</div>}
        </section>
    )
}