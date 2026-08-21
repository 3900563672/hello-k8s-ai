import { useEffect, useRef, useState } from 'react'
import {
    ArrowLeft,
    Bot,
    CheckCircle2,
    KeyRound,
    Loader2,
    MessageCircle,
    Save,
    Send,
    Settings2,
    X,
} from 'lucide-react'
import {
    fetchAIOpsChatMessages,
    fetchAIOpsSettings,
    streamAIOpsChat,
    updateAIOpsSettings,
} from '@/api/endpoints/aiopsApi'
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

type PanelView = 'chat' | 'settings'
type SaveState = 'idle' | 'saving' | 'saved' | 'error'

/**
 * 全局 AI 助手浮窗（#110 阶段三/四）：
 * 右下角气泡 → 对话面板；SSE 流式回答 + 工具步骤指示器；
 * 面板内「配置」入口可运行时设置模型 / 地址 / API Key——key 只存服务端内存
 * （重启后由环境变量恢复），前端仅显示掩码状态，localStorage 不含密钥。
 */
export function AiChatWidget() {
    const [open, setOpen] = useState(false)
    const [view, setView] = useState<PanelView>('chat')
    const [session, setSession] = useState<ChatSession>(loadSession)
    const [input, setInput] = useState('')
    const [busy, setBusy] = useState(false)
    const [toolSteps, setToolSteps] = useState<ToolStep[]>([])
    const [errorText, setErrorText] = useState<string | null>(null)
    const listRef = useRef<HTMLDivElement | null>(null)

    // 面板配置（#110 阶段四）：form 为本地编辑态，configured 为服务端掩码态。
    const [settingsLoading, setSettingsLoading] = useState(false)
    const [settingsError, setSettingsError] = useState<string | null>(null)
    const [saveState, setSaveState] = useState<SaveState>('idle')
    const [configured, setConfigured] = useState({ keyConfigured: false, model: '', baseUrl: '' })
    const [form, setForm] = useState({ model: '', baseUrl: '', apiKey: '' })

    useEffect(() => {
        saveSession(session)
    }, [session])

    useEffect(() => {
        const list = listRef.current
        if (list) list.scrollTop = list.scrollHeight
    }, [session.messages, toolSteps, open])
    // 打开面板时拉取服务端历史（#112 阶段 D 读侧）：只回填空会话，不覆盖本地新消息；
    // 拉取失败静默降级（本地会话仍可用）。
    useEffect(() => {
        if (!open || view !== 'chat') return
        let cancelled = false
        void fetchAIOpsChatMessages(session.sessionId, 50)
            .then((envelope) => {
                if (cancelled || envelope.data.length === 0) return
                setSession((current) => {
                    if (current.messages.length > 0) return current
                    return {
                        ...current,
                        messages: envelope.data.map((message) => ({
                            role: message.role,
                            content: message.content,
                        })),
                    }
                })
            })
            .catch(() => {
                // 历史拉取失败静默降级
            })
        return () => {
            cancelled = true
        }
    }, [open, view, session.sessionId])

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

    const openSettings = async () => {
        setView('settings')
        setSettingsLoading(true)
        setSettingsError(null)
        setSaveState('idle')
        try {
            const envelope = await fetchAIOpsSettings()
            const settings = envelope.data
            setConfigured({
                keyConfigured: settings.keyConfigured,
                model: settings.model,
                baseUrl: settings.baseUrl,
            })
            setForm((current) => ({
                ...current,
                model: current.model || settings.model,
                baseUrl: current.baseUrl || settings.baseUrl,
            }))
        } catch (error) {
            setSettingsError(error instanceof Error ? error.message : '读取配置失败')
        } finally {
            setSettingsLoading(false)
        }
    }

    const saveSettings = async () => {
        setSaveState('saving')
        setSettingsError(null)
        const payload: { apiKey?: string; model?: string; baseUrl?: string } = {}
        if (form.apiKey.trim()) payload.apiKey = form.apiKey.trim()
        if (form.model.trim()) payload.model = form.model.trim()
        if (form.baseUrl.trim()) payload.baseUrl = form.baseUrl.trim()
        try {
            const envelope = await updateAIOpsSettings(payload)
            const settings = envelope.data
            setConfigured({
                keyConfigured: settings.keyConfigured,
                model: settings.model,
                baseUrl: settings.baseUrl,
            })
            setForm((current) => ({ ...current, apiKey: '' }))
            setSaveState('saved')
        } catch (error) {
            setSaveState('error')
            setSettingsError(error instanceof Error ? error.message : '保存失败')
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
                        {view === 'chat' ? (
                            <>
                                <Bot className="h-4 w-4 text-[#7CAEFF]" />
                                <span className="text-[13px] font-semibold text-[#E8EEF7]">AI 助手</span>
                                <span className="ml-auto text-[10px] text-[#5A6778]">密钥仅存服务端</span>
                                <button
                                    type="button"
                                    onClick={() => void openSettings()}
                                    aria-label="AI 助手设置"
                                    className="ml-1 flex h-6 w-6 items-center justify-center rounded-md text-[#5A6778] transition-colors hover:bg-white/[0.06] hover:text-[#8C99AC]"
                                >
                                    <Settings2 className="h-3.5 w-3.5" />
                                </button>
                            </>
                        ) : (
                            <>
                                <button
                                    type="button"
                                    onClick={() => setView('chat')}
                                    aria-label="返回对话"
                                    className="flex h-6 w-6 items-center justify-center rounded-md text-[#8C99AC] transition-colors hover:bg-white/[0.06] hover:text-[#E8EEF7]"
                                >
                                    <ArrowLeft className="h-4 w-4" />
                                </button>
                                <Bot className="h-4 w-4 text-[#7CAEFF]" />
                                <span className="text-[13px] font-semibold text-[#E8EEF7]">AI 助手设置</span>
                            </>
                        )}
                    </div>

                    {view === 'chat' ? (
                        <>
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
                        </>
                    ) : (
                        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
                            {settingsLoading ? (
                                <div className="flex items-center gap-2 py-10 text-[12px] text-[#5A6778]">
                                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                                    读取配置…
                                </div>
                            ) : (
                                <div className="space-y-4">
                                    <p className="text-[11px] leading-5 text-[#5A6778]">
                                        配置保存在服务端内存，重启后由环境变量恢复；API Key 不回显、不进入前端存储。
                                    </p>
                                    <label className="block">
                                        <span className="mb-1 block text-[11px] text-[#8C99AC]">模型名</span>
                                        <input
                                            value={form.model}
                                            onChange={(event) => setForm({ ...form, model: event.target.value })}
                                            placeholder={configured.model || '例如 gpt-4o-mini'}
                                            className="h-9 w-full rounded-lg border border-white/[0.08] bg-white/[0.03] px-3 text-[12px] text-[#E8EEF7] outline-none placeholder:text-[#5A6778] focus:border-[#5B8CFF]/50"
                                        />
                                    </label>
                                    <label className="block">
                                        <span className="mb-1 block text-[11px] text-[#8C99AC]">API 地址</span>
                                        <input
                                            value={form.baseUrl}
                                            onChange={(event) => setForm({ ...form, baseUrl: event.target.value })}
                                            placeholder={configured.baseUrl || '例如 https://api.openai.com/v1'}
                                            className="h-9 w-full rounded-lg border border-white/[0.08] bg-white/[0.03] px-3 text-[12px] text-[#E8EEF7] outline-none placeholder:text-[#5A6778] focus:border-[#5B8CFF]/50"
                                        />
                                    </label>
                                    <label className="block">
                                        <span className="mb-1 flex items-center gap-1.5 text-[11px] text-[#8C99AC]">
                                            <KeyRound className="h-3 w-3" />
                                            API Key
                                            {configured.keyConfigured && (
                                                <span className="flex items-center gap-0.5 text-[11px] text-emerald-300">
                                                    <CheckCircle2 className="h-3 w-3" />
                                                    已配置
                                                </span>
                                            )}
                                        </span>
                                        <input
                                            type="password"
                                            value={form.apiKey}
                                            onChange={(event) => setForm({ ...form, apiKey: event.target.value })}
                                            placeholder={configured.keyConfigured ? '留空保持不变' : '输入新的 API Key'}
                                            autoComplete="off"
                                            className="h-9 w-full rounded-lg border border-white/[0.08] bg-white/[0.03] px-3 text-[12px] text-[#E8EEF7] outline-none placeholder:text-[#5A6778] focus:border-[#5B8CFF]/50"
                                        />
                                    </label>
                                    {settingsError && (
                                        <p className="rounded-lg border border-red-400/20 bg-red-400/[0.06] px-3 py-2 text-[11px] leading-5 text-red-200/90">
                                            {settingsError}
                                        </p>
                                    )}
                                    <div className="flex items-center gap-3">
                                        <button
                                            type="button"
                                            onClick={() => void saveSettings()}
                                            disabled={saveState === 'saving'}
                                            className="flex h-9 items-center gap-1.5 rounded-lg bg-[#5B8CFF] px-4 text-[12px] font-medium text-white transition-opacity disabled:opacity-50"
                                        >
                                            {saveState === 'saving' ? (
                                                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                                            ) : (
                                                <Save className="h-3.5 w-3.5" />
                                            )}
                                            {saveState === 'saving' ? '保存中…' : '保存'}
                                        </button>
                                        {saveState === 'saved' && (
                                            <span className="text-[11px] text-emerald-300">已保存 ✓</span>
                                        )}
                                    </div>
                                </div>
                            )}
                        </div>
                    )}
                </div>
            )}
        </>
    )
}