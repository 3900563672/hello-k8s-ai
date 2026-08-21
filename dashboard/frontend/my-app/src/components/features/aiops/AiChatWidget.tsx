import { useEffect, useRef, useState } from 'react'
import {
    Bot,
    CheckCircle2,
    Loader2,
    MessageCircle,
    Send,
    X,
} from 'lucide-react'
import { streamAIOpsChat } from '@/api/endpoints/aiopsApi'
import { ApiRequestError } from '@/api/client'
import { cn } from '@/lib/utils'

/** 会话本地存储：仅聊天记录与会话 id，不含任何密钥（#110 阶段三）。 */
const STORAGE_KEY = 'hello-k8s-ai.aiops-chat-session'

interface ChatMessage {
    role: 'user' | 'assistant'
    content: string
    pending?: boolean
}

interface ToolStep {
    name: string
    phase: 'start' | 'end'
}

interface ChatSession {
    sessionId: string
    messages: ChatMessage[]
}

function loadSession(): ChatSession {
    try {
        const raw = localStorage.getItem(STORAGE_KEY)
        if (raw) {
            const parsed = JSON.parse(raw) as ChatSession
            if (parsed && typeof parsed.sessionId === 'string' && Array.isArray(parsed.messages)) {
                return parsed
            }
        }
    } catch {
        // 忽略损坏的本地会话
    }
    return { sessionId: `chat-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`, messages: [] }
}

function saveSession(session: ChatSession) {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(session))
    } catch {
        // 存储满/隐私模式下降级为内存会话
    }
}

/**
 * 全局 AI 助手浮窗（#110 阶段三）：
 * 右下角气泡 → 对话面板；SSE 流式回答 + 工具步骤指示器；
 * 会话跨页面保留（本地存储，不含 key）。密钥只存在于服务端。
 */
export function AiChatWidget() {
    const [open, setOpen] = useState(false)
    const [session, setSession] = useState<ChatSession>(loadSession)
    const [input, setInput] = useState('')
    const [busy, setBusy] = useState(false)
    const [toolSteps, setToolSteps] = useState<ToolStep[]>([])
    const [errorText, setErrorText] = useState<string | null>(null)
    const listRef = useRef<HTMLDivElement | null>(null)

    useEffect(() => {
        saveSession(session)
    }, [session])

    useEffect(() => {
        const list = listRef.current
        if (list) list.scrollTop = list.scrollHeight
    }, [session.messages, toolSteps, open])

    const appendAssistant = (delta: string) => {
        setSession((current) => {
            const messages = [...current.messages]
            const last = messages[messages.length - 1]
            if (last && last.role === 'assistant' && last.pending) {
                messages[messages.length - 1] = { ...last, content: last.content + delta }
            } else {
                messages.push({ role: 'assistant', content: delta, pending: true })
            }
            return { ...current, messages }
        })
    }

    const finishAssistant = () => {
        setSession((current) => {
            const messages = [...current.messages]
            const last = messages[messages.length - 1]
            if (last && last.role === 'assistant' && last.pending) {
                messages[messages.length - 1] = { ...last, pending: false }
            }
            return { ...current, messages }
        })
    }

    const send = async () => {
        const text = input.trim()
        if (!text || busy) return
        setInput('')
        setErrorText(null)
        setBusy(true)
        setToolSteps([])
        const sessionId = session.sessionId
        setSession((current) => ({
            ...current,
            messages: [...current.messages, { role: 'user' as const, content: text }],
        }))
        try {
            await streamAIOpsChat(text, sessionId, {
                onLifecycle: (phase, error) => {
                    if (phase === 'end') {
                        if (error) {
                            setErrorText(`回答中断：${error}`)
                        }
                        setBusy(false)
                        finishAssistant()
                    }
                },
                onTool: (name, phase) => {
                    setToolSteps((steps) => [...steps, { name, phase }])
                },
                onText: (delta) => appendAssistant(delta),
            })
        } catch (error) {
            if (error instanceof ApiRequestError && error.status === 404) {
                setErrorText('AIOps 未启用：后端需配置 AIOPS_ENABLED=true 与 API Key。')
            } else {
                setErrorText(error instanceof Error ? error.message : '对话请求失败')
            }
            setBusy(false)
        }
    }

    const activeTools = toolSteps.filter((step) => step.phase === 'start').map((step) => step.name)
    const doneTools = new Set(toolSteps.filter((step) => step.phase === 'end').map((step) => step.name))

    return (
        <>
            <button
                type="button"
                onClick={() => setOpen((value) => !value)}
                aria-label={open ? '关闭 AI 助手' : '打开 AI 助手'}
                className="fixed bottom-6 right-6 z-[110] flex h-12 w-12 items-center justify-center rounded-full bg-[#5B8CFF] text-white shadow-lg shadow-[#5B8CFF]/25 transition-transform hover:scale-105"
            >
                {open ? <X className="h-5 w-5" /> : <MessageCircle className="h-5 w-5" />}
            </button>

            {open && (
                <div className="fixed bottom-24 right-6 z-[110] flex h-[min(60vh,560px)] w-[min(400px,calc(100vw-3rem))] flex-col overflow-hidden rounded-2xl border border-white/[0.08] bg-[#0A0E15]/95 shadow-2xl shadow-black/50 backdrop-blur">
                    <div className="flex items-center gap-2 border-b border-white/[0.06] px-4 py-3">
                        <Bot className="h-4 w-4 text-[#7CAEFF]" />
                        <span className="text-[13px] font-semibold text-[#E8EEF7]">AI 助手</span>
                        <span className="ml-auto text-[10px] text-[#5A6778]">密钥仅存服务端</span>
                    </div>

                    <div ref={listRef} className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-3">
                        {session.messages.length === 0 && !errorText && (
                            <p className="pt-6 text-center text-[12px] leading-5 text-[#5A6778]">
                                问点什么吧，例如「当前集群什么情况？」
                                <br />
                                回答基于 AIOps 切面总结与警戒，不读原始事件。
                            </p>
                        )}
                        {session.messages.map((message, index) => (
                            <div
                                key={`${index}-${message.content.length}`}
                                className={cn(
                                    'max-w-[85%] whitespace-pre-wrap rounded-xl px-3 py-2 text-[12px] leading-5',
                                    message.role === 'user'
                                        ? 'ml-auto bg-[#5B8CFF]/20 text-[#D6E2FF]'
                                        : 'border border-white/[0.06] bg-white/[0.03] text-[#C6D0DE]',
                                )}
                            >
                                {message.content}
                                {message.pending && <Loader2 className="mt-1 inline h-3 w-3 animate-spin text-[#5B8CFF]" />}
                            </div>
                        ))}
                        {busy && activeTools.length > 0 && (
                            <div className="space-y-1.5 rounded-xl border border-white/[0.06] bg-white/[0.02] px-3 py-2">
                                {activeTools.map((name) => (
                                    <div key={name} className="flex items-center gap-2 text-[11px] text-[#8C99AC]">
                                        {doneTools.has(name) ? (
                                            <CheckCircle2 className="h-3 w-3 text-emerald-300" />
                                        ) : (
                                            <Loader2 className="h-3 w-3 animate-spin text-[#5B8CFF]" />
                                        )}
                                        {name}
                                    </div>
                                ))}
                            </div>
                        )}
                        {errorText && (
                            <p className="rounded-xl border border-red-400/20 bg-red-400/[0.06] px-3 py-2 text-[11px] leading-5 text-red-200/90">
                                {errorText}
                            </p>
                        )}
                    </div>

                    <div className="border-t border-white/[0.06] p-3">
                        <div className="flex items-end gap-2">
                            <textarea
                                value={input}
                                onChange={(event) => setInput(event.target.value)}
                                onKeyDown={(event) => {
                                    if (event.key === 'Enter' && !event.shiftKey) {
                                        event.preventDefault()
                                        void send()
                                    }
                                }}
                                rows={1}
                                placeholder="输入问题，Enter 发送"
                                className="max-h-28 min-h-[36px] flex-1 resize-none rounded-lg border border-white/[0.08] bg-white/[0.03] px-3 py-2 text-[12px] text-[#E8EEF7] outline-none placeholder:text-[#5A6778] focus:border-[#5B8CFF]/50"
                            />
                            <button
                                type="button"
                                onClick={() => void send()}
                                disabled={busy || !input.trim()}
                                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[#5B8CFF] text-white transition-opacity disabled:opacity-40"
                                aria-label="发送"
                            >
                                <Send className="h-4 w-4" />
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </>
    )
}
