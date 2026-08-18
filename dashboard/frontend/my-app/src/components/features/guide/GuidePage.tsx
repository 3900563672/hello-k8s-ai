import type { ReactNode } from 'react'
import {
    BookOpen,
    Braces,
    BrainCircuit,
    FileClock,
    Gauge,
    Route,
    Server,
    ShieldCheck,
    SlidersHorizontal,
    Users,
    Waves,
} from 'lucide-react'
import { Link } from 'react-router-dom'
import { ConfigFormSection } from '@/components/features/config/forms/ConfigFormParts'
import {
    PRESET_MODEL_TEMPLATES,
    PRESET_NODE_TEMPLATES,
    PRESET_ORCHESTRATOR_TEMPLATES,
    PRESET_TENANT_TEMPLATES,
    PRESET_TRAFFIC_TEMPLATES,
} from '@/lib/constants/presetTemplates'

interface FieldRowProps {
    name: string
    unit?: string
    defaultValue?: string
    children: ReactNode
}

function FieldRow({ name, unit, defaultValue, children }: FieldRowProps) {
    return (
        <div className="flex flex-col gap-1 border-b border-[#1B2634]/60 py-2.5 last:border-b-0 sm:flex-row sm:items-baseline sm:gap-3">
            <div className="w-44 shrink-0">
                <code className="text-xs text-[#C9D4E3]">{name}</code>
                {unit && <span className="ml-1.5 text-[10px] text-[#596579]">{unit}</span>}
            </div>
            <div className="min-w-0 flex-1 text-xs leading-5 text-[#8B97A8]">{children}</div>
            {defaultValue && (
                <div className="shrink-0 text-right">
                    <span className="rounded border border-[#2A3548] bg-[#111722] px-1.5 py-0.5 font-mono text-[10px] text-[#8CB8F8]">
                        默认 {defaultValue}
                    </span>
                </div>
            )}
        </div>
    )
}

type ParamOwner = '用户可配置' | '系统常量' | '开发测试'

interface ParamRow {
    label: string
    value: string
    owner: ParamOwner
}

const ownerStyles: Record<ParamOwner, string> = {
    用户可配置: 'border-emerald-500/20 bg-emerald-500/5 text-[#72CFA2]',
    系统常量: 'border-[#5B8CFF]/20 bg-[#5B8CFF]/10 text-[#8CB8F8]',
    开发测试: 'border-amber-400/20 bg-amber-400/5 text-amber-300',
}

function ParamsTable({ rows }: { rows: ParamRow[] }) {
    return (
        <div className="overflow-hidden rounded-xl border border-[#232323] bg-[#0B1018]">
            <table className="w-full border-collapse text-left">
                <thead>
                    <tr className="border-b border-[#263244] bg-[#0D131C] text-[10px] text-[#6F7B8E]">
                        <th className="px-4 py-2.5 font-medium">参数</th>
                        <th className="px-4 py-2.5 font-medium">值</th>
                        <th className="px-4 py-2.5 text-right font-medium">归属</th>
                    </tr>
                </thead>
                <tbody>
                    {rows.map((row) => (
                        <tr key={row.label} className="border-b border-[#1B2634]/60 last:border-b-0">
                            <td className="px-4 py-2.5 font-mono text-[11px] text-[#C9D4E3]">{row.label}</td>
                            <td className="px-4 py-2.5 font-mono text-[11px] text-[#AAB6C5]">{row.value}</td>
                            <td className="px-4 py-2.5 text-right">
                                <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] ${ownerStyles[row.owner]}`}>
                                    {row.owner}
                                </span>
                            </td>
                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    )
}


const systemParams: ParamRow[] = [
    { label: '模型 gpuUnits / maxConcurrency', value: '1 / 1', owner: '用户可配置' },
    { label: '模型 absoluteScore / coldStartMs', value: '100 / 0 ms', owner: '用户可配置' },
    { label: '性能画像 prefillBase / prefillPerToken / decodePerToken', value: '50 ms / 500 µs / 20 ms', owner: '用户可配置' },
    { label: '节点 gpu / maxConcurrency', value: '1 G / 1', owner: '用户可配置' },
    { label: '租户 priority / qps', value: 'P3 / 0', owner: '用户可配置' },
    { label: '租户 TTFT 扩容 / 队列扩容', value: '500 ms / 100', owner: '用户可配置' },
    { label: '租户 TTFT 缩容 / 队列缩容', value: '200 ms / 30', owner: '用户可配置' },
    { label: '编排 扩容冷却 / 缩容冷却', value: '60 s / 120 s', owner: '用户可配置' },
    { label: '编排 扩容步长', value: '10 副本/轮', owner: '用户可配置' },
    { label: '编排 allowScaleToZero / 副本范围', value: 'false / 1..10', owner: '用户可配置' },
    { label: '控制器 冷启动打分基准 / 衰减 / 权重下限', value: '60 s / 0.2 / 0.7', owner: '系统常量' },
    { label: '控制器 指标新鲜度', value: '30 s', owner: '系统常量' },
    { label: '控制器 编排同步周期', value: '10 s', owner: '系统常量' },
    { label: '控制器 SimulationClock 倍速范围', value: '1..20', owner: '系统常量' },
    { label: '模拟器 请求模板 promptTokens / outputTokens', value: '500 / 200', owner: '系统常量' },
    { label: '模拟器 服务时间噪声', value: '±20%', owner: '系统常量' },
    { label: '模拟器 状态 tick 间隔', value: '5 s', owner: '系统常量' },
    { label: '模拟器 租约 15 / 10 / 2 s', value: 'Lease / Renew / Retry', owner: '系统常量' },
    { label: '模拟器 单步物化请求上限', value: '100000', owner: '系统常量' },
    { label: '后端 快照间隔 / 保留时长', value: '30 s / 30 天', owner: '系统常量' },
    { label: '后端 SSE 心跳', value: '15 s', owner: '系统常量' },
    { label: '后端 时间线 limit 默认 / 上限', value: '200 / 1000', owner: '系统常量' },
    { label: '后端 HTTP 读超时', value: '15 s', owner: '系统常量' },
]

export function GuidePage() {
    return (
        <div className="relative h-full overflow-auto bg-[#05070A] text-[#E8EEF7]">
            <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_56%_6%,rgba(91,140,255,.08),transparent_28%)]" />
            <main className="relative mx-auto h-full w-full max-w-[1500px] px-5 py-6 lg:px-8 lg:py-8">
                <header className="flex shrink-0 flex-col gap-4 border-b border-white/[0.07] pb-5 lg:flex-row lg:items-end lg:justify-between">
                    <div>
                        <div className="flex items-center gap-2 text-[10px] font-medium uppercase tracking-[0.16em] text-[#6B788C]">
                            <BookOpen className="h-3.5 w-3.5 text-[#7CAEFF]" />
                            Guide / 参数速查
                        </div>
                        <h1 className="mt-3 text-2xl font-semibold tracking-[-0.025em] text-[#F0F5FB]">
                            填写指南
                        </h1>
                        <p className="mt-1.5 text-[11px] text-[#657286]">
                            字段含义、性能单位、系统硬编码参数与模拟条件下的取值建议
                        </p>
                    </div>
                    <Link
                        to="/config"
                        className="inline-flex h-8 items-center gap-2 rounded-md border border-white/[0.08] bg-white/[0.025] px-3 text-[10px] text-[#AAB6C8] outline-none transition duration-150 hover:bg-white/[0.06] hover:text-white"
                    >
                        <SlidersHorizontal className="h-3.5 w-3.5" />
                        返回配置中心
                    </Link>
                </header>

                <div className="mt-6 space-y-4">
                    <ConfigFormSection
                        title="标识符生成规则"
                        description="创建资源时，系统标识由显示名称自动生成，与配置中心预览保持一致。"
                        action={<Braces className="h-4 w-4 text-[#5E9EFF]" />}
                    >
                        <div className="space-y-0">
                            <FieldRow name="NFKC 规范化">全角字符与兼容字符先统一（如全角空格、括号、数字）。</FieldRow>
                            <FieldRow name="转小写">字母统一转为小写，避免大小写差异造成标识冲突。</FieldRow>
                            <FieldRow name="替换分隔符">非字母数字字符（空格、下划线、标点等）替换为 -，再去除首尾的 -。</FieldRow>
                            <FieldRow name="类型前缀">空名称回退为 {`{prefix}-{时间戳36进制}`}；前缀分别为 model / node / tenant / orch。</FieldRow>
                            <FieldRow name="冲突后缀">与现有资源重名时追加 -2、-3…，直到全局唯一。</FieldRow>
                            <FieldRow name="中文保留">中文属于字母类字符（<code>{'\\p{L}'}</code>），会原样保留，例如“轻量在线推理” → 轻量在线推理。</FieldRow>
                            <FieldRow name="策略标识">策略名由引用对象拼接：租户-模型 / 租户-节点 / 模型-节点，冲突同样追加后缀。</FieldRow>
                            <FieldRow name="创建后固定">系统标识即资源名称；后续重命名只修改显示名称，不影响标识。</FieldRow>
                        </div>
                    </ConfigFormSection>

                    <ConfigFormSection
                        title="模型"
                        description="定义推理模型的资源需求、承载能力与性能画像。"
                        action={<BrainCircuit className="h-4 w-4 text-[#5E9EFF]" />}
                    >
                        <div className="space-y-0">
                            <FieldRow name="gpuUnits" unit="G" defaultValue="1">模型占用的显存需求，调度时受节点总显存约束。</FieldRow>
                            <FieldRow name="maxConcurrency" defaultValue="1">单个预热副本可同时服务的最大请求并发。</FieldRow>
                            <FieldRow name="absoluteScore" defaultValue="100">单个预热副本的理想能力分，Orchestrator 用其比较扩容候选。</FieldRow>
                            <FieldRow name="coldStartMs" unit="ms" defaultValue="0">冷启动耗时；越长则扩容生效越慢，控制器打分衰减越多。</FieldRow>
                            <FieldRow name="prefillBaseMs" unit="ms" defaultValue="50">Prefill 基础延时。单请求预填充 ≈ prefillBaseMs + prefillPerTokenUs × 500 / 1000（Simulator 固定 promptTokens=500）。</FieldRow>
                            <FieldRow name="prefillPerTokenUs" unit="µs" defaultValue="500">每 token 预填充延时（微秒）。</FieldRow>
                            <FieldRow name="decodePerTokenMs" unit="ms" defaultValue="20">每 token 解码延时。单请求解码 ≈ decodePerTokenMs × 200（outputTokens=200）。</FieldRow>
                        </div>
                        <PresetList
                            title="预置模板示例"
                            rows={PRESET_MODEL_TEMPLATES.map((template) => ({
                                id: template.id,
                                name: template.name,
                                summary: `${template.data.gpuUnits} G · ${template.data.maxConcurrency} 并发 · 能力分 ${template.data.absoluteScore} · 冷启动 ${template.data.coldStartMs} ms`,
                            }))}
                        />
                    </ConfigFormSection>

                    <ConfigFormSection
                        title="租户"
                        description="定义业务租户的优先级、基准流量与弹性阈值。"
                        action={<Users className="h-4 w-4 text-[#5E9EFF]" />}
                    >
                        <div className="space-y-0">
                            <FieldRow name="priority" defaultValue="P3">P1 最紧急（红色语义）到 P5 最低（灰色语义）；高优先级租户在争抢副本时优先获得容量。</FieldRow>
                            <FieldRow name="qps" unit="QPS" defaultValue="0">基准流量；0 表示无固定基准，按实际请求触发。</FieldRow>
                            <FieldRow name="ttftThresholdMs" unit="ms" defaultValue="500">TTFT 扩容阈值：首 token 延时超过该值且持续时触发扩容。</FieldRow>
                            <FieldRow name="queueThreshold" defaultValue="100">队列扩容阈值：排队请求数超过该值且持续时触发扩容。</FieldRow>
                            <FieldRow name="ttftScaleDownThresholdMs" unit="ms" defaultValue="200">TTFT 缩容阈值，必须严格小于扩容阈值（500）。</FieldRow>
                            <FieldRow name="queueScaleDownThreshold" defaultValue="30">队列缩容阈值，必须严格小于扩容阈值（100）。</FieldRow>
                        </div>
                        <PresetList
                            title="预置模板示例"
                            rows={PRESET_TENANT_TEMPLATES.map((template) => ({
                                id: template.id,
                                name: template.name,
                                summary: `${template.data.priority} · ${template.data.qps} QPS · TTFT ${template.data.ttftThresholdMs}/${template.data.ttftScaleDownThresholdMs} ms · Queue ${template.data.queueThreshold}/${template.data.queueScaleDownThreshold}`,
                            }))}
                        />
                    </ConfigFormSection>

                    <ConfigFormSection
                        title="节点"
                        description="定义可参与调度的计算资源。"
                        action={<Server className="h-4 w-4 text-[#5E9EFF]" />}
                    >
                        <div className="space-y-0">
                            <FieldRow name="gpu" unit="G" defaultValue="1">节点总显存。</FieldRow>
                            <FieldRow name="maxConcurrency" defaultValue="1">节点可同时服务的最大请求并发。</FieldRow>
                            <FieldRow name="与模型并发的关系">节点可承载的模型副本数 ≈ min(⌊gpu / gpuUnits⌋, ⌊maxConcurrency / 模型 maxConcurrency⌋)；两者同时受约束，先定节点池再定模型副本。</FieldRow>
                        </div>
                        <PresetList
                            title="预置模板示例"
                            rows={PRESET_NODE_TEMPLATES.map((template) => ({
                                id: template.id,
                                name: template.name,
                                summary: `${template.data.gpu} G · ${template.data.maxConcurrency} 并发`,
                            }))}
                        />
                    </ConfigFormSection>

                    <ConfigFormSection
                        title="编排策略"
                        description="定义租户的扩缩容冷却、副本范围与缩容行为。"
                        action={<Route className="h-4 w-4 text-[#5E9EFF]" />}
                    >
                        <div className="space-y-0">
                            <FieldRow name="scaleUpCooldownSeconds" unit="s" defaultValue="60">扩容冷却：同方向扩容动作的最小间隔。</FieldRow>
                            <FieldRow name="scaleDownCooldownSeconds" unit="s" defaultValue="120">缩容冷却：同方向缩容动作的最小间隔。</FieldRow>
                            <FieldRow name="maxScaleUpBatch" defaultValue="10">单次扩容步长：每轮最多补的副本数；填 0 使用默认 10，配合扩容冷却形成批次节奏。</FieldRow>
                            <FieldRow name="minReplicas / maxReplicas" defaultValue="1..∞">副本范围；maxReplicas 填 0 表示不限制（模拟器无网关，接受任意 QPS，扩到容量上限为止），正整数时要求 minReplicas ≤ maxReplicas。</FieldRow>
                            <FieldRow name="allowScaleToZero" defaultValue="false">允许缩到零：空闲时可将副本缩至 0（minReplicas 仍需 ≥ 1 通过校验）。</FieldRow>
                        </div>
                        <PresetList
                            title="预置模板示例"
                            rows={PRESET_ORCHESTRATOR_TEMPLATES.map((template) => ({
                                id: template.id,
                                name: template.name,
                                summary: `${template.data.scaleUpCooldownSeconds}/${template.data.scaleDownCooldownSeconds} s · ${template.data.minReplicas}..${template.data.maxReplicas === 0 ? '∞' : template.data.maxReplicas} · 缩零 ${template.data.allowScaleToZero ? '允许' : '禁止'}`,
                            }))}
                        />
                    </ConfigFormSection>

                    <ConfigFormSection
                        title="策略"
                        description="租户、模型与节点的 Allow / Deny 关系，Controller 据此拉起 Simulator。"
                        action={<ShieldCheck className="h-4 w-4 text-[#5E9EFF]" />}
                    >
                        <div className="space-y-0">
                            <FieldRow name="tenantmodelpolicy">租户 → 模型：决定租户能否使用某个模型。</FieldRow>
                            <FieldRow name="tenantnodepolicy">租户 → 节点：决定租户可调度的计算节点范围。</FieldRow>
                            <FieldRow name="modelnodepolicy">模型 → 节点：决定模型可运行的节点；未配置时沿用租户节点范围。</FieldRow>
                            <FieldRow name="effect">Allow 允许使用并参与调度与扩容；Deny 禁止使用，优先级高于 Allow。</FieldRow>
                            <FieldRow name="引用不可变">创建后引用对象不可修改（CRD XValidation 强制，变更需删除重建）；effect 可随时调整。</FieldRow>
                        </div>
                    </ConfigFormSection>

                    <ConfigFormSection
                        title="流量"
                        description="模板控制点语义、叠加方式与逻辑时间。"
                        action={<Waves className="h-4 w-4 text-[#5E9EFF]" />}
                    >
                        <div className="space-y-0">
                            <FieldRow name="控制点">x 为逻辑时间（秒，从 T+0 起），y 为该时刻的绝对 QPS 增量；曲线在控制点之间插值。</FieldRow>
                            <FieldRow name="叠加 = 纯 QPS 加法">多个模板叠加到同一租户时，同一时刻的 QPS 直接相加，不做归一化或缩放。</FieldRow>
                            <FieldRow name="逻辑时间">受 SimulationClock 倍速影响：rate=10 时 1 秒真实时间推进 10 秒逻辑时间；倍速范围 1..20。</FieldRow>
                        </div>
                        <PresetList
                            title="预置模板示例"
                            rows={PRESET_TRAFFIC_TEMPLATES.map((template) => {
                                const peak = Math.max(0, ...template.controlPoints.map((point) => point.y))
                                const duration = Math.max(0, ...template.controlPoints.map((point) => point.x))
                                return {
                                    id: template.id,
                                    name: template.name,
                                    summary: `峰值 ${peak} QPS · 时长 ${duration} s · ${template.controlPoints.length} 个控制点`,
                                }
                            })}
                        />
                    </ConfigFormSection>

                    <ConfigFormSection
                        title="系统参数速查"
                        description="前端默认值与各组件硬编码参数；归属标注为系统常量时不可通过表单修改。"
                        action={<Gauge className="h-4 w-4 text-[#5E9EFF]" />}
                    >
                        <ParamsTable rows={systemParams} />
                    </ConfigFormSection>

                    <ConfigFormSection
                        title="模拟条件下怎么填"
                        description="容量、阈值与性能画像的取值思路。"
                        action={<FileClock className="h-4 w-4 text-[#5E9EFF]" />}
                    >
                        <div className="space-y-0">
                            <FieldRow name="容量估算">模型 gpuUnits × 期望副本数 ≤ 可用节点 gpu 总和；模型 maxConcurrency × 副本数 ≤ 节点 maxConcurrency 总和。</FieldRow>
                            <FieldRow name="副本吞吐换算">单副本吞吐 ≈ 模型 maxConcurrency ÷ 平均服务时长；平均服务时长 = prefillBaseMs + prefillPerTokenUs×0.5 + decodePerTokenMs×200（毫秒）。例：model-lite（50 + 500×0.5 + 20×200 = 4300 ms）单副本 ≈ 16 ÷ 4.3 ≈ 3.7 QPS。</FieldRow>
                            <FieldRow name="所需副本估算">副本数 ≈ QPS × 平均服务时长 ÷ maxConcurrency；例如 400 QPS × 4.3s ÷ 16 ≈ 108 副本。队列持续增长说明目标容量远小于负载需求，先按此公式扩节点与副本。</FieldRow>
                            <FieldRow name="节点能放多少副本">单节点可承载副本 = min(⌊gpu ÷ gpuUnits⌋, ⌊节点 maxConcurrency ÷ 模型 maxConcurrency⌋)；实例副本总数 = 各可用节点之和。扩容被节点容量挡住时 Orchestrator 返回 no_feasible_placement（属正常容量不足，不是错误）。</FieldRow>
                            <FieldRow name="无限流量与天花板">maxReplicas 填 0 表示副本数不设上限；模拟器无网关、接受任意 QPS，副本可扩到节点配置容量为止。模拟资源由节点/模型配置决定，真实上限只受 Docker Desktop 宿主资源约束，一般到不了。</FieldRow>
                            <FieldRow name="扩容节奏">高负载下按队列缺口批量扩容：一次决策最多补 maxScaleUpBatch 副本（默认 10，填 0 表示默认），扩容冷却（默认 60s）作为批次间隔；扩到目标副本数的时间 ≈ 缺口 ÷ 每批上限 × 冷却。想更快可调大步长或调小 scaleUpCooldownSeconds。</FieldRow>
                            <FieldRow name="QPS 与并发">目标 QPS × 平均服务时长（秒）≈ 所需并发；排队请求持续增长说明并发不足，应扩容或降低 QPS。</FieldRow>
                            <FieldRow name="冷启动窗口">coldStartMs 越大扩容生效越慢；控制器以 60 s 为基准打分，超过后权重按每 60 s 衰减 0.2、下限 0.7。延迟敏感租户建议小模型 + 小冷启动。</FieldRow>
                            <FieldRow name="阈值语义">TTFT 阈值衡量首 token 延时（ms），Queue 阈值衡量排队请求数；缩容阈值应明显低于扩容阈值（如 500/200、100/30），避免反复抖动。</FieldRow>
                            <FieldRow name="性能画像">无实测数据时保持 50 / 500 / 20 默认值；模拟结果偏快或偏慢时，再按实测校准这三个参数。</FieldRow>
                        </div>
                    </ConfigFormSection>
                </div>
            </main>
        </div>
    )
}

interface PresetListProps {
    title: string
    rows: Array<{ id: string; name: string; summary: string }>
}

function PresetList({ title, rows }: PresetListProps) {
    return (
        <div className="mt-4 overflow-hidden rounded-xl border border-[#232323] bg-[#0D131C]">
            <div className="border-b border-[#1B2634] px-4 py-2.5 text-[10px] font-medium text-[#6F7B8E]">
                {title}
            </div>
            <div className="divide-y divide-[#1B2634]/60">
                {rows.map((row) => (
                    <div key={row.id} className="flex flex-col gap-1 px-4 py-2.5 sm:flex-row sm:items-baseline sm:gap-3">
                        <span className="inline-flex w-36 shrink-0 items-center gap-1.5 text-xs font-medium text-[#DCE4EE]">
                            <span className="h-1 w-1 rounded-full bg-[#5B8CFF]" />
                            {row.name}
                        </span>
                        <span className="min-w-0 flex-1 font-mono text-[11px] text-[#8B97A8]">{row.summary}</span>
                    </div>
                ))}
            </div>
        </div>
    )
}