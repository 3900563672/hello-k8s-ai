import { useEffect, useState } from 'react'
import { AlertTriangle, ExternalLink, Gauge, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'

const GRAFANA_BASE = '/grafana'
const GRAFANA_OVERVIEW = `${GRAFANA_BASE}/d/hello-k8s-ai-overview?kiosk`

type GrafanaState = 'checking' | 'ready' | 'unavailable'

function useGrafanaHealth(): GrafanaState {
    const [state, setState] = useState<GrafanaState>('checking')

    useEffect(() => {
        let cancelled = false
        const check = async () => {
            try {
                const response = await fetch(`${GRAFANA_BASE}/api/health`, {
                    headers: { Accept: 'application/json' },
                })
                if (!cancelled) {
                    setState(response.ok ? 'ready' : 'unavailable')
                }
            } catch {
                if (!cancelled) setState('unavailable')
            }
        }
        void check()
        const timer = setInterval(() => void check(), 30_000)
        return () => {
            cancelled = true
            clearInterval(timer)
        }
    }, [])

    return state
}

export function MonitorPage() {
    const [reloadKey, setReloadKey] = useState(0)
    const grafanaState = useGrafanaHealth()

    return (
        <div className="relative h-full overflow-auto bg-[#05070A] text-[#E8EEF7]">
            <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_56%_6%,rgba(91,140,255,.08),transparent_28%)]" />
            <main className="relative mx-auto flex h-full min-h-0 w-full max-w-[1500px] flex-col px-5 py-6 lg:px-8 lg:py-8">
                <header className="flex shrink-0 flex-col gap-4 border-b border-white/[0.07] pb-5 lg:flex-row lg:items-end lg:justify-between">
                    <div>
                        <div className="flex items-center gap-2 text-[10px] font-medium uppercase tracking-[0.16em] text-[#6B788C]">
                            <Gauge className="h-3.5 w-3.5 text-[#7CAEFF]" />
                            Grafana / Observability
                            <span className={`inline-flex h-1.5 w-1.5 rounded-full ${
                                grafanaState === 'ready'
                                    ? 'bg-emerald-300 shadow-[0_0_7px_rgba(110,231,183,.55)]'
                                    : grafanaState === 'unavailable'
                                        ? 'bg-amber-300'
                                        : 'bg-[#465267]'
                            }`} />
                        </div>
                        <h1 className="mt-3 text-2xl font-semibold tracking-[-0.025em] text-[#F0F5FB]">
                            监控面板
                        </h1>
                        <p className="mt-1.5 text-[11px] text-[#657286]">
                            Grafana 统一视图 · Prometheus 指标与 Jaeger 链路均通过 Dashboard 单入口访问
                        </p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                        <a
                            href={`${GRAFANA_BASE}/`}
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex h-8 items-center gap-2 rounded-md border border-white/[0.08] bg-white/[0.025] px-3 text-[10px] text-[#AAB6C8] outline-none transition duration-150 hover:bg-white/[0.06] hover:text-white"
                        >
                            <ExternalLink className="h-3.5 w-3.5" />
                            新窗口打开
                        </a>
                        <Button
                            type="button"
                            variant="outline"
                            onClick={() => setReloadKey((key) => key + 1)}
                            className="h-8 gap-2 border-white/[0.08] bg-white/[0.025] px-3 text-[10px] text-[#AAB6C8] hover:bg-white/[0.06] hover:text-white"
                        >
                            <RefreshCw className="h-3.5 w-3.5" />
                            刷新
                        </Button>
                    </div>
                </header>

                {grafanaState === 'unavailable' && (
                    <div className="mt-4 flex items-start gap-2 rounded-lg border border-amber-300/10 bg-amber-300/[0.035] px-3 py-2 text-[9px] leading-4 text-amber-100/75">
                        <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0 text-amber-300/70" />
                        Grafana 暂不可用：请确认 `make cluster-up` 已完成、Backend 的 GRAFANA_URL 指向
                        hello-k8s-ai-grafana:3000，并检查 Backend 日志。
                    </div>
                )}

                <div className="relative mt-4 min-h-[420px] min-w-0 flex-1 overflow-hidden rounded-xl border border-white/[0.07] bg-[#0A0E15]/90">
                    <iframe
                        key={reloadKey}
                        src={GRAFANA_OVERVIEW}
                        title="Grafana 监控面板"
                        className="h-full w-full bg-transparent"
                        allow="fullscreen"
                    />
                </div>
            </main>
        </div>
    )
}