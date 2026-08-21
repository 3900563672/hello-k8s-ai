import { useEffect, useMemo, useRef, useState } from 'react'
import {
    AlertTriangle,
    Boxes,
    Database,
    Network,
    Sparkles,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import {
    Sheet,
    SheetContent,
    SheetHeader,
    SheetTitle,
} from '@/components/ui/sheet'
import type {
    BackendEvent,
    BackendNode,
    BackendPod,
    OverviewData,
    ResourceRef,
    TenantTraffic,
} from '@/types/trace.types'
import type { KubernetesCondition } from '@/types/config.types'

/**
 * Agent 结构化分级（预留）：未来 AI Ops 写入 Pod/Node/Tenant 上的
 * agentVerdict 字段后，气泡外圈会按分级着色；未接入前不渲染外圈。
 */
export type AgentGrade = 'normal' | 'odd' | 'problematic'

export interface AgentVerdict {
    grade?: AgentGrade
    score?: number
    summary?: string
    updatedAt?: string
}

type Health = 'ok' | 'warn' | 'crit' | 'unknown'
type BubbleKind = 'node' | 'pod' | 'tenant' | 'header'

const HEALTH_COLOR: Record<Health, string> = {
    ok: '#34D399',
    warn: '#F0A33B',
    crit: '#F87171',
    unknown: '#5A6778',
}

const HEALTH_LABEL: Record<Health, string> = {
    ok: '健康',
    warn: '关注',
    crit: '严重',
    unknown: '未知',
}

const AGENT_RING: Record<AgentGrade, string> = {
    normal: 'rgba(52,211,153,0.85)',
    odd: 'rgba(240,163,59,0.85)',
    problematic: 'rgba(248,113,113,0.85)',
}

const AGENT_LABEL: Record<AgentGrade, string> = {
    normal: '正常',
    odd: '奇怪',
    problematic: '有问题',
}

const KIND_LABEL: Record<BubbleKind, string> = {
    node: '节点',
    pod: 'Pod',
    tenant: '租户',
    header: '分组',
}

const FIELD_W = 960
const POD_D = 52
const POD_X_STEP = 66
const PODS_PER_ROW = 14
const POD_ROW_H = 74
const NODES_PER_ROW = 5
const NODE_D = 132
const NODE_X_STEP = 160
const NODE_ROW_H = 180
const TENANT_D = 132
const TOP_PAD = 44
const MIN_CANVAS_H = 430
const HEADER_H = 32
const GROUP_GAP = 26
const NODE_LABEL_BUDGET = 16

function podHealth(pod: BackendPod): Health {
    if (pod.ready) return 'ok'
    const phase = pod.phase.toLowerCase()
    if (['failed', 'error', 'crashloopbackoff', 'unknown'].includes(phase)) return 'crit'
    return 'warn'
}

function nodeHealth(node: BackendNode): Health {
    if (node.ready && node.schedulable) return 'ok'
    if (!node.ready) return 'crit'
    return 'warn'
}

function tenantHealth(tenant: TenantTraffic): Health {
    const phase = (tenant.runtimePhase ?? '').toLowerCase()
    if (tenant.readyReplicaCount > 0 && phase !== 'failed' && phase !== 'error') return 'ok'
    if (phase === 'failed' || phase === 'error') return 'crit'
    return 'warn'
}

function agentOf(item: { agentVerdict?: AgentVerdict }): AgentVerdict | undefined {
    return item.agentVerdict
}

function refKey(ref: ResourceRef): string {
    return `${ref.kind}:${ref.namespace ?? ''}:${ref.name}`
}

/** 显示宽度：CJK 按双字符计，近似等宽字体的实际占位。 */
function displayWidth(value: string): number {
    let width = 0
    for (const ch of value) width += ch.charCodeAt(0) > 255 ? 2 : 1
    return width
}

/** 名称适配气泡宽度：超长时保留前缀 + 后缀（Pod 后缀是唯一哈希），中间省略。 */
function fitName(name: string, budget: number): string {
    if (displayWidth(name) <= budget) return name
    const ellipsis = '…'
    const tail = name.slice(-5)
    const headBudget = budget - displayWidth(ellipsis) - displayWidth(tail)
    let head = ''
    let used = 0
    for (const ch of name.slice(0, -tail.length)) {
        const width = ch.charCodeAt(0) > 255 ? 2 : 1
        if (used + width > headBudget) break
        head += ch
        used += width
    }
    return head + ellipsis + tail
}

export interface BubbleItem {
    id: string
    kind: BubbleKind
    name: string
    x: number
    y: number
    radius: number | [number, number]
    health: Health
    ring?: string
    label?: string
    alias?: string
    nodeName?: string
    tenant?: string
    groupKey?: Health
    data: BackendNode | BackendPod | TenantTraffic
}

interface BubbleLayout {
    items: BubbleItem[]
    height: number
}

/**
 * 单类视图布局：节点视图为顶部紧凑网格；Pod 视图按健康状态分组（严重/关注/健康）
 * 从上到下排布，组内每行固定数量；租户视图为底部一行。
 */
function layoutBubbles(
    nodes: BackendNode[],
    pods: BackendPod[],
    tenants: TenantTraffic[],
    kind: Exclude<BubbleKind, 'header'>,
    aliases: Map<string, string>,
): BubbleLayout {
    const items: BubbleItem[] = []

    if (kind === 'node') {
        const rows = Math.max(1, Math.ceil(nodes.length / NODES_PER_ROW))
        const nodeTop = Math.max(TOP_PAD + NODE_D / 2, (MIN_CANVAS_H - rows * NODE_ROW_H) / 2)
        nodes.forEach((node, index) => {
            const podCount = pods.filter((pod) => pod.nodeName === node.ref.name).length
            const col = index % NODES_PER_ROW
            const row = Math.floor(index / NODES_PER_ROW)
            const agent = agentOf(node as { agentVerdict?: AgentVerdict })
            items.push({
                id: refKey(node.ref),
                kind: 'node',
                name: node.ref.name,
                x: 70 + col * NODE_X_STEP,
                y: nodeTop + row * NODE_ROW_H,
                radius: NODE_D,
                health: nodeHealth(node),
                ring: agent?.grade ? AGENT_RING[agent.grade] : undefined,
                label: `${fitName(node.ref.name, NODE_LABEL_BUDGET)}\n${podCount} Pods`,
                data: node,
            })
        })
        return { items, height: Math.max(MIN_CANVAS_H, nodeTop * 2 + rows * NODE_ROW_H) }
    }

    if (kind === 'tenant') {
        const tenantCount = Math.max(1, tenants.length)
        const tenantTop = Math.max(TOP_PAD, (MIN_CANVAS_H - 170) / 2)
        tenants.forEach((tenant, index) => {
            const agent = agentOf(tenant as { agentVerdict?: AgentVerdict })
            const podCount = pods.filter((pod) => pod.tenant === tenant.tenant.name).length
            items.push({
                id: refKey(tenant.tenant),
                kind: 'tenant',
                name: tenant.displayName || tenant.tenant.name,
                x: tenantCount === 1 ? FIELD_W / 2 : 140 + (index / (tenantCount - 1)) * (FIELD_W - 280),
                y: tenantTop + 70,
                radius: TENANT_D,
                health: tenantHealth(tenant),
                ring: agent?.grade ? AGENT_RING[agent.grade] : undefined,
                label: `${fitName(tenant.displayName || tenant.tenant.name, NODE_LABEL_BUDGET)}\n${podCount} Pods`,
                data: tenant,
            })
        })
        return { items, height: MIN_CANVAS_H }
    }

    const groups: Array<{ key: Health; label: string }> = [
        { key: 'crit', label: HEALTH_LABEL.crit },
        { key: 'warn', label: HEALTH_LABEL.warn },
        { key: 'ok', label: HEALTH_LABEL.ok },
    ]
    let y = TOP_PAD
    for (const group of groups) {
        const list = pods.filter((pod) => podHealth(pod) === group.key)
        if (list.length === 0) continue
        items.push({
            id: `header:${group.key}`,
            kind: 'header',
            name: `${group.label} ${list.length}`,
            x: 20,
            y,
            radius: 0,
            health: group.key,
            groupKey: group.key,
            data: {} as BackendPod,
        })
        y += HEADER_H
        list.forEach((pod, index) => {
            const col = index % PODS_PER_ROW
            const row = Math.floor(index / PODS_PER_ROW)
            const agent = agentOf(pod as { agentVerdict?: AgentVerdict })
            items.push({
                id: refKey(pod.ref),
                kind: 'pod',
                name: pod.ref.name,
                x: 30 + col * POD_X_STEP,
                y: y + row * POD_ROW_H,
                radius: POD_D,
                health: podHealth(pod),
                ring: agent?.grade ? AGENT_RING[agent.grade] : undefined,
                alias: aliases.get(refKey(pod.ref)),
                label: aliases.get(refKey(pod.ref)) ?? pod.ref.name,
                nodeName: pod.nodeName,
                tenant: pod.tenant,
                data: pod,
            })
        })
        y += Math.ceil(list.length / PODS_PER_ROW) * POD_ROW_H + GROUP_GAP
    }
    return { items, height: y + TOP_PAD }
}

function rowHeightOf(items: BubbleItem[]): number {
    if (items.some((item) => item.kind === 'header')) return HEADER_H
    if (items.some((item) => item.kind === 'node' || item.kind === 'tenant')) return NODE_ROW_H
    return POD_ROW_H
}

function bubbleTitle(item: BubbleItem): string {
    const lines = [item.alias ?? item.name, KIND_LABEL[item.kind] + ' · ' + HEALTH_LABEL[item.health]]
    if (item.kind === 'pod') lines.push('真实名称：' + item.name)
    if (item.kind === 'pod' && item.nodeName) lines.push('节点：' + item.nodeName)
    if (item.kind === 'pod' && item.tenant) lines.push('租户：' + item.tenant)
    const agent = agentOf(item.data as { agentVerdict?: AgentVerdict })
    if (agent?.grade) {
        lines.push('Agent：' + AGENT_LABEL[agent.grade] + (typeof agent.score === 'number' ? ' (' + agent.score.toFixed(2) + ')' : ''))
    }
    return lines.join('\n')
}

function conditionsSummary(conditions: KubernetesCondition[]): Array<{ type: string; status: string; reason?: string }> {
    return conditions.map((condition) => ({
        type: condition.type,
        status: condition.status,
        reason: condition.reason,
    }))
}

function DetailRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
    return (
        <div className="flex items-start justify-between gap-3 py-1.5">
            <span className="shrink-0 text-[12px] text-[#657184]">{label}</span>
            <span className={`min-w-0 break-all text-right text-[12px] text-[#C6D0DE] ${mono ? 'font-mono' : ''}`}>
                {value || '—'}
            </span>
        </div>
    )
}

function AgentSection({ agent }: { agent?: AgentVerdict }) {
    if (!agent?.grade) {
        return (
            <div className="rounded-xl border border-dashed border-white/[0.09] bg-white/[0.015] p-3">
                <div className="flex items-center gap-2 text-[12px] font-medium text-[#8C99AC]">
                    <Sparkles className="h-3.5 w-3.5 text-[#7F96EF]" />
                    Agent 解析
                </div>
                <p className="mt-2 text-[12px] leading-4 text-[#5A6778]">
                    AI Ops 接入后，这里会给出该对象的结构化判定（正常 / 奇怪 / 有问题）与分数。
                </p>
            </div>
        )
    }
    return (
        <div className="rounded-xl border border-[#6E8BFF]/15 bg-[#6E8BFF]/[0.055] p-3">
            <div className="flex items-center gap-2 text-[12px] font-medium text-[#9CAFFF]">
                <Sparkles className="h-3.5 w-3.5 text-[#7F96EF]" />
                Agent 解析
                <Badge
                    variant="outline"
                    className={
                        'h-5 px-1.5 text-[12px] ' +
                        (agent.grade === 'normal'
                            ? 'border-emerald-400/20 bg-emerald-400/10 text-emerald-300'
                            : agent.grade === 'odd'
                                ? 'border-amber-400/20 bg-amber-400/10 text-amber-200'
                                : 'border-red-400/20 bg-red-400/10 text-red-300')
                    }
                >
                    {AGENT_LABEL[agent.grade]}
                </Badge>
                {typeof agent.score === 'number' && (
                    <span className="font-mono text-[12px] text-[#C6D0DE]">{agent.score.toFixed(2)}</span>
                )}
            </div>
            {agent.summary && (
                <p className="mt-2 text-[13px] leading-[1.6] text-[#B7C2D0]">{agent.summary}</p>
            )}
            {agent.updatedAt && (
                <p className="mt-2 text-[11px] text-[#5A6778]">更新于 {new Date(agent.updatedAt).toLocaleString()}</p>
            )}
        </div>
    )
}

/** 反向关联列表：按 nodeName / tenant 字段反查 Pod（数据天然支持，O(n) 过滤）。 */
function withAliases(pods: BackendPod[], aliases: Map<string, string>): Array<{ pod: BackendPod; alias?: string }> {
    return pods.map((pod) => ({ pod, alias: aliases.get(refKey(pod.ref)) }))
}

function AssociatedPods({
    pods,
    onSelectPod,
}: {
    pods: Array<{ pod: BackendPod; alias?: string }>
    onSelectPod: (podId: string) => void
}) {
    const [keyword, setKeyword] = useState('')
    const ordered = useMemo(() => {
        const rank: Record<Health, number> = { crit: 0, warn: 1, ok: 2, unknown: 3 }
        return [...pods].sort(
            (a, b) =>
                rank[podHealth(a.pod)] - rank[podHealth(b.pod)] ||
                (a.alias ?? a.pod.ref.name).localeCompare(b.alias ?? b.pod.ref.name, undefined, { numeric: true }),
        )
    }, [pods])
    const filtered = useMemo(() => {
        const key = keyword.trim().toLowerCase()
        if (!key) return ordered
        return ordered.filter((entry) => {
            const alias = entry.alias?.toLowerCase() ?? ''
            const name = entry.pod.ref.name.toLowerCase()
            return alias.includes(key) || name.includes(key)
        })
    }, [ordered, keyword])
    const shown = filtered.slice(0, 60)
    return (
        <section className="rounded-xl border border-white/[0.07] bg-[#0A0E15] p-3.5">
            <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2 text-[12px] font-medium text-[#CFD7E3]">
                    <Boxes className="h-3.5 w-3.5 text-[#7F96EF]" />
                    关联 Pod
                    <span className="font-mono text-[11px] text-[#5A6778]">{pods.length}</span>
                </div>
                <input
                    value={keyword}
                    onChange={(event) => setKeyword(event.target.value)}
                    placeholder="编号或名称"
                    className="h-6 w-32 rounded-md border border-white/[0.07] bg-white/[0.02] px-2 text-[11px] text-[#C6D0DE] outline-none placeholder:text-[#4C5868]"
                />
            </div>
            <div className="mt-2 max-h-72 space-y-1 overflow-y-auto pr-1">
                {shown.map((entry) => (
                    <button
                        key={refKey(entry.pod.ref)}
                        type="button"
                        onClick={() => onSelectPod(refKey(entry.pod.ref))}
                        className="flex w-full items-center gap-2 rounded-lg bg-white/[0.025] px-2.5 py-1.5 text-left transition-colors hover:bg-white/[0.06]"
                    >
                        <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: HEALTH_COLOR[podHealth(entry.pod)] }} />
                        <span className="shrink-0 font-mono text-[11px] text-[#8FA0B8]">{entry.alias ?? '—'}</span>
                        <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-[#C6D0DE]">{entry.pod.ref.name}</span>
                        <span className="shrink-0 text-[11px] text-[#5A6778]">
                            {entry.pod.phase}
                            {entry.pod.ready ? '' : ' · 未就绪'}
                        </span>
                    </button>
                ))}
                {filtered.length === 0 && <p className="text-[12px] text-[#5A6778]">无匹配 Pod</p>}
                {shown.length < filtered.length && (
                    <p className="pt-1 text-[11px] text-[#4C5868]">
                        仅显示前 {shown.length} 个（共 {filtered.length} 个），可在上方输入框继续筛选
                    </p>
                )}
            </div>
        </section>
    )
}

function BubbleDetailDrawer({
    item,
    events,
    pods,
    aliases,
    onSelectPod,
    onClose,
}: {
    item: BubbleItem
    events: BackendEvent[]
    pods: BackendPod[]
    aliases: Map<string, string>
    onSelectPod: (podId: string) => void
    onClose: () => void
}) {
    const related = events
        .filter((event) => refKey(event.regarding) === item.id)
        .slice(-6)
        .reverse()

    const health = item.health
    const healthBadge =
        health === 'ok'
            ? 'border-emerald-400/20 bg-emerald-400/10 text-emerald-300'
            : health === 'warn'
                ? 'border-amber-400/20 bg-amber-400/10 text-amber-200'
                : health === 'crit'
                    ? 'border-red-400/20 bg-red-400/10 text-red-300'
                    : 'border-white/[0.08] bg-white/[0.03] text-[#8C99AC]'

    return (
        <Sheet open onOpenChange={(open) => { if (!open) onClose() }}>
            <SheetContent
                side="right"
                className="w-full overflow-y-auto border-l border-white/[0.08] bg-[#080B11] p-0 text-[#E8EEF7] sm:max-w-[430px]"
            >
                <SheetHeader className="border-b border-white/[0.07] px-5 py-4">
                    <div className="flex items-center gap-2.5">
                        <Badge variant="outline" className="h-6 px-2 text-[12px] text-[#9EB2FF]">
                            {KIND_LABEL[item.kind]}
                        </Badge>
                        <SheetTitle className="truncate text-sm font-semibold text-[#F0F5FB]">
                            {item.alias ? `${item.alias} · ${item.name}` : item.name}
                        </SheetTitle>
                        <Badge variant="outline" className={healthBadge + ' h-5 px-1.5 text-[12px]'}>
                            {HEALTH_LABEL[health]}
                        </Badge>
                    </div>
                </SheetHeader>

                <div className="space-y-4 px-5 py-4">
                    <AgentSection agent={agentOf(item.data as { agentVerdict?: AgentVerdict })} />

                    <section className="rounded-xl border border-white/[0.07] bg-[#0A0E15] p-3.5">
                        <div className="flex items-center gap-2 text-[12px] font-medium text-[#CFD7E3]">
                            <Database className="h-3.5 w-3.5 text-[#7F96EF]" />
                            基本信息
                        </div>
                        <div className="mt-2 divide-y divide-white/[0.05]">
                            {item.kind === 'node' && (() => {
                                const node = item.data as BackendNode
                                return (
                                    <>
                                        <DetailRow label="角色" value={node.role} />
                                        <DetailRow label="Phase" value={node.phase} />
                                        <DetailRow label="可调度" value={node.schedulable ? '是' : '否'} />
                                        <DetailRow label="可用区" value={node.zone ?? ''} />
                                        <DetailRow label="版本" value={node.version ?? ''} />
                                        <DetailRow label="内网 IP" value={node.internalIP ?? ''} mono />
                                    </>
                                )
                            })()}
                            {item.kind === 'pod' && (() => {
                                const pod = item.data as BackendPod
                                return (
                                    <>
                                        <DetailRow label="真实名称" value={pod.ref.name} mono />
                                        <DetailRow label="Phase" value={pod.phase} />
                                        <DetailRow label="就绪" value={pod.ready ? '是' : '否'} />
                                        <DetailRow label="节点" value={pod.nodeName ?? ''} />
                                        <DetailRow label="Pod IP" value={pod.podIP ?? ''} mono />
                                        <DetailRow label="租户" value={pod.tenant ?? ''} />
                                        <DetailRow label="模型" value={pod.model ?? ''} />
                                        <DetailRow label="启动时间" value={pod.startTime ? new Date(pod.startTime).toLocaleString() : ''} />
                                    </>
                                )
                            })()}
                            {item.kind === 'tenant' && (() => {
                                const tenant = item.data as TenantTraffic
                                return (
                                    <>
                                        <DetailRow label="优先级" value={tenant.priority} />
                                        <DetailRow label="请求 QPS" value={String(tenant.requestedQPS)} mono />
                                        <DetailRow label="分配 QPS" value={String(tenant.allocatedQPS)} mono />
                                        <DetailRow label="就绪副本" value={String(tenant.readyReplicaCount)} mono />
                                        <DetailRow label="运行态" value={tenant.runtimePhase ?? ''} />
                                        <DetailRow
                                            label="TTFT"
                                            value={tenant.performance.avgTTFT ? `${tenant.performance.avgTTFT.value} ${tenant.performance.avgTTFT.unit ?? 'ms'}` : ''}
                                            mono
                                        />
                                        <DetailRow
                                            label="队列"
                                            value={tenant.performance.avgQueue ? String(tenant.performance.avgQueue.value) : ''}
                                            mono
                                        />
                                        <DetailRow label="采样数" value={String(tenant.performance.sampleCount)} mono />
                                    </>
                                )
                            })()}
                        </div>
                    </section>

                    {item.kind === 'node' && (
                        <AssociatedPods
                            pods={withAliases(
                                pods.filter((pod) => pod.nodeName === (item.data as BackendNode).ref.name),
                                aliases,
                            )}
                            onSelectPod={onSelectPod}
                        />
                    )}
                    {item.kind === 'tenant' && (
                        <AssociatedPods
                            pods={withAliases(
                                pods.filter((pod) => pod.tenant === (item.data as TenantTraffic).tenant.name),
                                aliases,
                            )}
                            onSelectPod={onSelectPod}
                        />
                    )}

                    <section className="rounded-xl border border-white/[0.07] bg-[#0A0E15] p-3.5">
                        <div className="flex items-center gap-2 text-[12px] font-medium text-[#CFD7E3]">
                            <Network className="h-3.5 w-3.5 text-[#7F96EF]" />
                            状态条件
                        </div>
                        <div className="mt-2 space-y-1.5">
                            {'conditions' in item.data && Array.isArray((item.data as { conditions?: KubernetesCondition[] }).conditions)
                                ? conditionsSummary((item.data as { conditions: KubernetesCondition[] }).conditions).map((condition) => (
                                    <div key={condition.type} className="flex items-center justify-between rounded-lg bg-white/[0.025] px-2.5 py-1.5">
                                        <span className="text-[12px] text-[#93A1B5]">{condition.type}</span>
                                        <span className="text-[12px] text-[#C6D0DE]">
                                            {condition.status}
                                            {condition.reason ? ` · ${condition.reason}` : ''}
                                        </span>
                                    </div>
                                ))
                                : <p className="text-[12px] text-[#5A6778]">无条件数据</p>}
                        </div>
                    </section>

                    {item.kind === 'pod' && (() => {
                        const pod = item.data as BackendPod
                        return (
                            <section className="rounded-xl border border-white/[0.07] bg-[#0A0E15] p-3.5">
                                <div className="flex items-center gap-2 text-[12px] font-medium text-[#CFD7E3]">
                                    <Boxes className="h-3.5 w-3.5 text-[#7F96EF]" />
                                    容器
                                </div>
                                <div className="mt-2 space-y-1.5">
                                    {pod.containers.map((container) => (
                                        <div key={container.name} className="flex items-center justify-between rounded-lg bg-white/[0.025] px-2.5 py-1.5">
                                            <span className="text-[12px] text-[#93A1B5]">{container.name}</span>
                                            <span className="text-[12px] text-[#C6D0DE]">
                                                {container.ready ? '就绪' : container.state}
                                                {container.reason ? ` · ${container.reason}` : ''}
                                                {container.restartCount > 0 ? ` · 重启 ${container.restartCount}` : ''}
                                            </span>
                                        </div>
                                    ))}
                                    {pod.containers.length === 0 && (
                                        <p className="text-[12px] text-[#5A6778]">无容器数据</p>
                                    )}
                                </div>
                            </section>
                        )
                    })()}

                    <section className="rounded-xl border border-white/[0.07] bg-[#0A0E15] p-3.5">
                        <div className="flex items-center gap-2 text-[12px] font-medium text-[#CFD7E3]">
                            <AlertTriangle className="h-3.5 w-3.5 text-[#7F96EF]" />
                            近期事件
                        </div>
                        <div className="mt-2 space-y-1.5">
                            {related.map((event) => (
                                <div key={event.id} className="rounded-lg bg-white/[0.025] px-2.5 py-1.5">
                                    <div className="flex items-center justify-between">
                                        <span className="text-[12px] font-medium text-[#93A1B5]">{event.reason}</span>
                                        <span className="text-[11px] text-[#5A6778]">
                                            {new Date(event.eventTime).toLocaleString()}
                                        </span>
                                    </div>
                                    <p className="mt-1 text-[12px] leading-4 text-[#6B788C]">{event.message}</p>
                                </div>
                            ))}
                            {related.length === 0 && (
                                <p className="text-[12px] text-[#5A6778]">该对象暂无关联事件</p>
                            )}
                        </div>
                    </section>
                </div>
            </SheetContent>
        </Sheet>
    )
}

function LegendDot({ color, label }: { color: string; label: string }) {
    return (
        <span className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full" style={{ backgroundColor: color }} />
            <span className="text-[12px] text-[#6B788C]">{label}</span>
        </span>
    )
}

export function ClusterBubbleField({ overview }: { overview: OverviewData | undefined }) {
    const nodes = useMemo(() => overview?.workloads.nodes ?? [], [overview])
    const pods = useMemo(() => overview?.workloads.pods ?? [], [overview])
    const tenants = useMemo(() => overview?.traffic.tenants ?? [], [overview])
    const events = useMemo(() => overview?.workloads.events ?? [], [overview])

    const [kindFilter, setKindFilter] = useState<Exclude<BubbleKind, 'header'>>('pod')
    const [healthFilter, setHealthFilter] = useState<'all' | Health>('all')
    const [tenantFilter, setTenantFilter] = useState('all')
    const [selected, setSelected] = useState<BubbleItem | null>(null)

    const tenantOptions = useMemo(
        () => [...new Set(pods.map((pod) => pod.tenant).filter((value): value is string => Boolean(value)))],
        [pods],
    )

    const podAliasById = useMemo(() => {
        const sorted = [...pods].sort((a, b) => a.ref.name.localeCompare(b.ref.name))
        const map = new Map<string, string>()
        sorted.forEach((pod, index) => {
            map.set(refKey(pod.ref), `Pod ${String(index + 1).padStart(3, '0')}`)
        })
        return map
    }, [pods])

    const layout = useMemo(
        () => layoutBubbles(nodes, pods, tenants, kindFilter, podAliasById),
        [nodes, pods, tenants, kindFilter, podAliasById],
    )

    /** 按容器实际宽度缩放 x 坐标（与原来 ECharts 画布自适应一致），行宽不足时保持可读下限。 */
    const fieldRef = useRef<HTMLDivElement>(null)
    const [fieldWidth, setFieldWidth] = useState(FIELD_W)
    useEffect(() => {
        const el = fieldRef.current
        if (!el) return
        const update = () => setFieldWidth(Math.max(840, el.clientWidth))
        update()
        const observer = new ResizeObserver(update)
        observer.observe(el)
        return () => observer.disconnect()
    }, [])
    const scale = fieldWidth / FIELD_W

    const podItemsById = useMemo(() => {
        const map = new Map<string, BubbleItem>()
        for (const pod of pods) {
            const item: BubbleItem = {
                id: refKey(pod.ref),
                kind: 'pod',
                name: pod.ref.name,
                x: 0,
                y: 0,
                radius: POD_D,
                health: podHealth(pod),
                alias: podAliasById.get(refKey(pod.ref)),
                nodeName: pod.nodeName,
                tenant: pod.tenant,
                data: pod,
            }
            map.set(item.id, item)
        }
        return map
    }, [pods])

    const visible = useMemo(() => {
        return layout.items.filter((item) => {
            if (item.kind === 'header') {
                return healthFilter === 'all' || item.health === healthFilter
            }
            if (healthFilter !== 'all' && item.health !== healthFilter) return false
            if (tenantFilter !== 'all') {
                if (item.kind === 'pod' && item.tenant !== tenantFilter) return false
                if (item.kind === 'tenant' && item.name !== tenantFilter) return false
            }
            return true
        })
    }, [layout, healthFilter, tenantFilter])

    /** 按 y 聚合为行：行内元素保持原有绝对坐标，行外内容由 content-visibility 跳过渲染。 */
    const rows = useMemo(() => {
        const byY = new Map<number, BubbleItem[]>()
        for (const item of visible) {
            const list = byY.get(item.y)
            if (list) list.push(item)
            else byY.set(item.y, [item])
        }
        return [...byY.entries()]
            .sort((a, b) => a[0] - b[0])
            .map(([y, items]) => ({ y, items }))
    }, [visible])

    const filterChip = (active: boolean) =>
        active
            ? 'border-[#5B8CFF]/40 bg-[#5B8CFF]/[0.12] text-[#C9DDFF]'
            : 'border-white/[0.07] bg-white/[0.02] text-[#8C99AC] hover:bg-white/[0.05] hover:text-[#D5DFEC]'

    return (
        <div className="flex flex-col gap-3">
            <div className="flex flex-wrap items-center gap-2">
                <div className="flex flex-wrap items-center gap-1.5">
                    {([
                        { value: 'node' as const, label: `节点 ${nodes.length}` },
                        { value: 'pod' as const, label: `Pod ${pods.length}` },
                        { value: 'tenant' as const, label: `租户 ${tenants.length}` },
                    ]).map((chip) => (
                        <button
                            key={chip.value}
                            type="button"
                            onClick={() => setKindFilter(chip.value)}
                            className={'h-7 rounded-lg border px-2.5 text-[12px] transition-colors ' + filterChip(kindFilter === chip.value)}
                        >
                            {chip.label}
                        </button>
                    ))}
                </div>

                <div className="mx-1 hidden h-4 w-px bg-white/[0.07] sm:block" />

                <div className="flex flex-wrap items-center gap-1.5">
                    {([
                        { value: 'all' as const, label: '全部状态' },
                        { value: 'ok' as const, label: '健康' },
                        { value: 'warn' as const, label: '关注' },
                        { value: 'crit' as const, label: '严重' },
                    ]).map((chip) => (
                        <button
                            key={chip.value}
                            type="button"
                            onClick={() => setHealthFilter(chip.value)}
                            className={'h-7 rounded-lg border px-2.5 text-[12px] transition-colors ' + filterChip(healthFilter === chip.value)}
                        >
                            {chip.label}
                        </button>
                    ))}
                </div>

                {tenantOptions.length > 0 && (
                    <>
                        <div className="mx-1 hidden h-4 w-px bg-white/[0.07] sm:block" />
                        <select
                            value={tenantFilter}
                            onChange={(event) => setTenantFilter(event.target.value)}
                            className="h-7 rounded-lg border border-white/[0.07] bg-[#0A0E15] px-2 text-[12px] text-[#8C99AC] outline-none"
                        >
                            <option value="all">全部租户</option>
                            {tenantOptions.map((tenant) => (
                                <option key={tenant} value={tenant}>{tenant}</option>
                            ))}
                        </select>
                    </>
                )}

                <div className="flex-1" />

                <div className="flex items-center gap-3 rounded-lg border border-white/[0.06] bg-white/[0.015] px-2.5 py-1.5">
                    <LegendDot color="#34D399" label="健康" />
                    <LegendDot color="#F0A33B" label="关注" />
                    <LegendDot color="#F87171" label="严重" />
                    <span className="hidden items-center gap-1.5 lg:flex">
                        <span className="h-2 w-2 rounded-full border-2 border-[#F0A33B]" />
                        <span className="text-[12px] text-[#6B788C]">外圈 = Agent 分级</span>
                    </span>
                </div>
            </div>

            <div
                ref={fieldRef}
                className="overflow-hidden rounded-xl border border-white/[0.07] bg-[#090D14]/90"
                style={{ height: Math.max(430, layout.height), position: 'relative', minWidth: 840 }}
            >
                {rows.map((row) => {
                    const height = rowHeightOf(row.items)
                    const isHeader = row.items.some((item) => item.kind === 'header')
                    const half = isHeader
                        ? 0
                        : row.items.some((item) => item.kind === 'pod')
                            ? POD_D / 2
                            : NODE_D / 2
                    const rowTop = row.y - (isHeader ? 8 : half)
                    const rowLeft = isHeader
                        ? 0
                        : Math.min(...row.items.map((item) => item.x * scale - half))
                    const rowWidth = isHeader
                        ? fieldWidth
                        : Math.max(...row.items.map((item) => item.x * scale + half)) - rowLeft
                    return (
                        <div
                            key={row.y}
                            style={{
                                position: 'absolute',
                                top: rowTop,
                                left: rowLeft,
                                width: rowWidth,
                                height,
                                contentVisibility: 'auto',
                                containIntrinsicSize: rowWidth + 'px ' + height + 'px',
                            }}
                        >
                            {row.items.map((item) => {
                                const diameter = typeof item.radius === 'number' ? item.radius : 0
                                if (item.kind === 'header') {
                                    return (
                                        <div
                                            key={item.id}
                                            style={{
                                                position: 'absolute',
                                                left: item.x * scale + 10 - rowLeft,
                                                top: 0,
                                                height: 14,
                                                lineHeight: '14px',
                                                color: HEALTH_COLOR[item.health],
                                                fontSize: 13,
                                                fontWeight: 600,
                                            }}
                                        >
                                            {item.name}
                                        </div>
                                    )
                                }
                                const darkText = item.health !== 'unknown'
                                return (
                                    <button
                                        key={item.id}
                                        type="button"
                                        title={bubbleTitle(item)}
                                        onClick={() => setSelected(item)}
                                        className="absolute flex items-center justify-center rounded-full font-semibold transition-transform duration-150 hover:scale-105"
                                        style={{
                                            left: item.x * scale - rowLeft - diameter / 2,
                                            top: item.y - rowTop - diameter / 2,
                                            width: diameter,
                                            height: diameter,
                                            backgroundColor: HEALTH_COLOR[item.health],
                                            color: darkText ? '#0A0E15' : '#E8EEF7',
                                            border: item.ring ? '3px solid ' + item.ring : '1px solid rgba(8,11,17,0.55)',
                                            boxShadow: item.ring ? '0 0 14px ' + item.ring : '0 0 4px rgba(0,0,0,0.35)',
                                        }}
                                    >
                                        <span
                                            className="max-w-full text-center font-semibold"
                                            style={{
                                                fontSize: item.kind === 'pod' ? 10 : 11,
                                                lineHeight: item.kind === 'pod' ? '12px' : '14px',
                                                whiteSpace: 'pre-line',
                                                overflow: 'hidden',
                                            }}
                                        >
                                            {item.kind === 'pod' ? (item.label ?? item.name) : item.label}
                                        </span>
                                    </button>
                                )
                            })}
                        </div>
                    )
                })}
            </div>
            <p className="text-[12px] text-[#4C5868]">
                画布随内容自动变长，Pod 全部平铺显示（Pod 001… 稳定编号，悬停 / 点开看真实名称）；节点与租户直接显示名称；节点 / 租户抽屉可反查全部 Pod 并按编号或名称筛选
            </p>

            {selected && (
                <BubbleDetailDrawer
                    item={selected}
                    events={events}
                    pods={pods}
                    aliases={podAliasById}
                    onSelectPod={(podId) => {
                        const podItem = podItemsById.get(podId)
                        if (podItem) setSelected(podItem)
                    }}
                    onClose={() => setSelected(null)}
                />
            )}
        </div>
    )
}
