import { useState } from 'react'
import {
    Check,
    CornerDownLeft,
    FlaskConical,
    Send,
    Sparkles,
} from 'lucide-react'
import { cn } from '@/lib/utils'

interface DemoParse {
    scene: string
    duration: string
    anchor: string
    templateCount: number
    nodeCount: number
}

interface DemoCommand {
    id: number
    rawInput: string
    parsed: DemoParse
    status: 'confirmed' | 'executing'
}

/**
 * 一句话起实验入口（契约先行）：后端 M2 意图执行未接入时，
 * 解析为本地规则演示（明确标注 DEMO，不写任何接口）；
 * 后端就绪后替换为真实解析→确认→GATE→执行链路，UI 结构不变。
 */
function demoParse(raw: string): DemoParse {
    const lower = raw.toLowerCase()
    let scene = '平稳基线'
    if (lower.includes('高峰') || lower.includes('突发') || lower.includes('burst') || lower.includes('peak')) {
        scene = '突发流量高峰'
    } else if (lower.includes('低谷') || lower.includes('低峰') || lower.includes('idle')) {
        scene = '低峰空闲'
    } else if (lower.includes('压测') || lower.includes('压力') || lower.includes('stress')) {
        scene = '压力测试'
    }

    let duration = '2 小时'
    const hourMatch = raw.match(/(\d+)\s*(?:小时|h|hour)/i)
    const minuteMatch = raw.match(/(\d+)\s*(?:分钟|min|minute)/i)
    if (hourMatch) duration = `${hourMatch[1]} 小时`
    else if (minuteMatch) duration = `${minuteMatch[1]} 分钟`

    let anchor = '立即开始'
    const anchorMatch = raw.match(/(?:美国时间|美东|美西)?\s*(\d{1,2})\s*点/)
    if (anchorMatch) anchor = `锚点 ${anchorMatch[1]} 点（实际立即开始，锚点仅作时间轴语义）`

    return { scene, duration, anchor, templateCount: 10, nodeCount: 5 }
}

export function CommandInput() {
    const [raw, setRaw] = useState('')
    const [preview, setPreview] = useState<DemoParse | null>(null)
    const [history, setHistory] = useState<DemoCommand[]>([])

    const submit = () => {
        const input = raw.trim()
        if (!input) return
        setPreview(demoParse(input))
    }

    const confirm = () => {
        if (!preview) return
        setHistory((items) => [
            ...items,
            { id: Date.now(), rawInput: raw.trim(), parsed: preview, status: 'confirmed' },
        ])
        setRaw('')
        setPreview(null)
    }

    return (
        <div className="rounded-xl border border-white/[0.06] bg-[#0A0E15]/60 p-4">
            <div className="flex items-center gap-2">
                <h3 className="flex items-center gap-1.5 text-[12px] font-semibold text-[#C6D0DE]">
                    <Sparkles className="h-3.5 w-3.5 text-[#7CAEFF]" />
                    一句话起实验
                </h3>
                <span className="rounded-full border border-white/[0.08] bg-white/[0.03] px-2 py-0.5 text-[10px] text-[#5A6778]">
                    DEMO（后端 M2 未接入，本地规则演示）
                </span>
            </div>

            <div className="mt-3 flex items-center gap-2">
                <div className="relative flex-1">
                    <FlaskConical className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[#4C5868]" />
                    <input
                        value={raw}
                        onChange={(event) => setRaw(event.target.value)}
                        onKeyDown={(event) => {
                            if (event.key === 'Enter') submit()
                        }}
                        placeholder="例如：美国时间 9 点开始，持续 2 小时，突发流量高峰场景"
                        className="w-full rounded-lg border border-white/[0.08] bg-[#05070A] py-2 pl-9 pr-3 text-[13px] text-[#E8EEF7] placeholder:text-[#4C5868] focus:border-[#5B8CFF]/50 focus:outline-none"
                    />
                </div>
                <button
                    type="button"
                    onClick={submit}
                    disabled={!raw.trim()}
                    className="inline-flex items-center gap-1.5 rounded-lg bg-[#5B8CFF]/15 px-3 py-2 text-[12px] font-medium text-[#9EB2FF] transition-colors hover:bg-[#5B8CFF]/25 disabled:cursor-not-allowed disabled:opacity-40"
                >
                    <CornerDownLeft className="h-3.5 w-3.5" />
                    解析
                </button>
            </div>

            {preview && (
                <div className="mt-3 rounded-lg border border-[#5B8CFF]/25 bg-[#5B8CFF]/[0.06] px-3 py-3">
                    <div className="flex items-center gap-2">
                        <span className="text-[11px] font-medium uppercase tracking-[0.12em] text-[#7CAEFF]">
                            解析预览
                        </span>
                        <span className="ml-auto text-[10px] text-[#5A6778]">确认后进入执行（演示）</span>
                    </div>
                    <div className="mt-2 grid grid-cols-2 gap-2 text-[12px] sm:grid-cols-3 lg:grid-cols-5">
                        <div>
                            <p className="text-[10px] text-[#5A6778]">场景</p>
                            <p className="mt-0.5 text-[#C6D0DE]">{preview.scene}</p>
                        </div>
                        <div>
                            <p className="text-[10px] text-[#5A6778]">持续时长</p>
                            <p className="mt-0.5 text-[#C6D0DE]">{preview.duration}</p>
                        </div>
                        <div>
                            <p className="text-[10px] text-[#5A6778]">时间锚点</p>
                            <p className="mt-0.5 text-[#C6D0DE]">{preview.anchor}</p>
                        </div>
                        <div>
                            <p className="text-[10px] text-[#5A6778]">模板选择</p>
                            <p className="mt-0.5 text-[#C6D0DE]">{preview.templateCount} 个全选</p>
                        </div>
                        <div>
                            <p className="text-[10px] text-[#5A6778]">节点选择</p>
                            <p className="mt-0.5 text-[#C6D0DE]">{preview.nodeCount} 个全选</p>
                        </div>
                    </div>
                    <button
                        type="button"
                        onClick={confirm}
                        className="mt-3 inline-flex items-center gap-1.5 rounded-lg bg-emerald-400/15 px-3 py-1.5 text-[12px] font-medium text-emerald-300 transition-colors hover:bg-emerald-400/25"
                    >
                        <Check className="h-3.5 w-3.5" />
                        确认并执行（演示）
                    </button>
                </div>
            )}

            {history.length > 0 && (
                <div className="mt-3 space-y-1.5">
                    {history.map((item) => (
                        <div
                            key={item.id}
                            className="flex items-center gap-2 rounded-lg border border-white/[0.05] bg-white/[0.02] px-3 py-2"
                        >
                            <Send className="h-3 w-3 shrink-0 text-[#5B8CFF]" />
                            <span className="truncate text-[12px] text-[#C6D0DE]">{item.rawInput}</span>
                            <span className={cn(
                                'ml-auto shrink-0 rounded-full px-2 py-0.5 text-[10px]',
                                item.status === 'executing'
                                    ? 'bg-[#5B8CFF]/15 text-[#9EB2FF]'
                                    : 'bg-emerald-400/15 text-emerald-300',
                            )}>
                                {item.status === 'executing' ? '执行中' : '已确认'}
                            </span>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}
