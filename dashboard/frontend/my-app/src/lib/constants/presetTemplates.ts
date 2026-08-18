import type { ModelFormValues } from '@/lib/validations/model.schema'
import type { NodeFormValues } from '@/lib/validations/node.schema'
import type { OrchestratorFormValues } from '@/lib/validations/orchestrator.schema'
import type { TenantFormValues } from '@/lib/validations/tenant.schema'
import type { ConfigTemplate } from '@/types/config.types'
import type { TrafficTemplate } from '@/types/traffic.types'

/**
 * 内置预置模板。所有数值均通过对应 zod schema 校验，可直接作为新建资源的初始值；
 * createdAt 使用固定 ISO 时间，保证展示稳定。预置模板只存在于内存 store，
 * 删除后刷新页面会重新出现，不会写入 Kubernetes。
 */
export const PRESET_MODEL_TEMPLATES: ConfigTemplate<ModelFormValues>[] = [
    {
        id: 'preset-model-lite',
        name: '轻量在线推理',
        preset: true,
        createdAt: '2026-01-01T00:00:00.000Z',
        data: {
            displayName: '轻量在线推理',
            gpuUnits: 8,
            maxConcurrency: 16,
            absoluteScore: 75,
            coldStartMs: 800,
            performance: {
                prefillBaseMs: 50,
                prefillPerTokenUs: 500,
                decodePerTokenMs: 20,
            },
        },
    },
    {
        id: 'preset-model-standard',
        name: '标准在线推理',
        preset: true,
        createdAt: '2026-01-01T00:00:00.000Z',
        data: {
            displayName: '标准在线推理',
            gpuUnits: 16,
            maxConcurrency: 32,
            absoluteScore: 100,
            coldStartMs: 1500,
            performance: {
                prefillBaseMs: 50,
                prefillPerTokenUs: 500,
                decodePerTokenMs: 20,
            },
        },
    },
    {
        id: 'preset-model-batch',
        name: '批量离线任务',
        preset: true,
        createdAt: '2026-01-01T00:00:00.000Z',
        data: {
            displayName: '批量离线任务',
            gpuUnits: 32,
            maxConcurrency: 64,
            absoluteScore: 60,
            coldStartMs: 5000,
            performance: {
                prefillBaseMs: 50,
                prefillPerTokenUs: 500,
                decodePerTokenMs: 20,
            },
        },
    },
]

export const PRESET_NODE_TEMPLATES: ConfigTemplate<NodeFormValues>[] = [
    {
        id: 'preset-node-gpu-pool',
        name: '高并发 GPU 池',
        preset: true,
        createdAt: '2026-01-01T00:00:00.000Z',
        data: {
            displayName: '高并发 GPU 池',
            gpu: 80,
            maxConcurrency: 128,
        },
    },
    {
        id: 'preset-node-standard',
        name: '标准 GPU 节点',
        preset: true,
        createdAt: '2026-01-01T00:00:00.000Z',
        data: {
            displayName: '标准 GPU 节点',
            gpu: 32,
            maxConcurrency: 48,
        },
    },
    {
        id: 'preset-node-edge',
        name: '边缘轻量节点',
        preset: true,
        createdAt: '2026-01-01T00:00:00.000Z',
        data: {
            displayName: '边缘轻量节点',
            gpu: 8,
            maxConcurrency: 16,
        },
    },
]

export const PRESET_TENANT_TEMPLATES: ConfigTemplate<TenantFormValues>[] = [
    {
        id: 'preset-tenant-core',
        name: '核心在线业务',
        preset: true,
        createdAt: '2026-01-01T00:00:00.000Z',
        data: {
            displayName: '核心在线业务',
            priority: 'P1',
            qps: 20,
            ttftThresholdMs: 800,
            queueThreshold: 150,
            ttftScaleDownThresholdMs: 300,
            queueScaleDownThreshold: 40,
        },
    },
    {
        id: 'preset-tenant-general',
        name: '一般在线业务',
        preset: true,
        createdAt: '2026-01-01T00:00:00.000Z',
        data: {
            displayName: '一般在线业务',
            priority: 'P3',
            qps: 10,
            ttftThresholdMs: 500,
            queueThreshold: 100,
            ttftScaleDownThresholdMs: 200,
            queueScaleDownThreshold: 30,
        },
    },
    {
        id: 'preset-tenant-batch',
        name: '离线分析批',
        preset: true,
        createdAt: '2026-01-01T00:00:00.000Z',
        data: {
            displayName: '离线分析批',
            priority: 'P5',
            qps: 0,
            ttftThresholdMs: 2000,
            queueThreshold: 300,
            ttftScaleDownThresholdMs: 800,
            queueScaleDownThreshold: 60,
        },
    },
]

export const PRESET_ORCHESTRATOR_TEMPLATES: ConfigTemplate<OrchestratorFormValues>[] = [
    {
        id: 'preset-orchestrator-core',
        name: '核心租户编排策略',
        preset: true,
        createdAt: '2026-01-01T00:00:00.000Z',
        data: {
            // 与预置租户“核心在线业务”对应；实际创建时以现有租户为准
            tenantName: '核心在线业务',
            scaleUpCooldownSeconds: 60,
            scaleDownCooldownSeconds: 120,
            allowScaleToZero: false,
            minReplicas: 1,
            maxReplicas: 0, // 0 = 无限制（模拟器无网关，接受任意 QPS，扩到容量上限为止）
            maxScaleUpBatch: 10, // 每轮最多补 10 副本（默认节奏）
        },
    },
    {
        id: 'preset-orchestrator-elastic',
        name: '弹性扩缩策略',
        preset: true,
        createdAt: '2026-01-01T00:00:00.000Z',
        data: {
            tenantName: '一般在线业务',
            scaleUpCooldownSeconds: 30,
            scaleDownCooldownSeconds: 90,
            allowScaleToZero: true,
            minReplicas: 1,
            maxReplicas: 20,
            maxScaleUpBatch: 20, // 弹性策略：每轮最多补 20 副本，快速吸收突发流量
        },
    },
    {
        id: 'preset-orchestrator-conservative',
        name: '保守稳定策略',
        preset: true,
        createdAt: '2026-01-01T00:00:00.000Z',
        data: {
            tenantName: '离线分析批',
            scaleUpCooldownSeconds: 120,
            scaleDownCooldownSeconds: 240,
            allowScaleToZero: false,
            minReplicas: 1,
            maxReplicas: 4,
            maxScaleUpBatch: 2, // 保守策略：每轮只补 2 副本，扩容更谨慎
        },
    },
]

/**
 * 预置流量模板：x 为逻辑时间（秒，从 T+0 起），y 为绝对 QPS 增量；
 * 叠加到租户时按纯 QPS 加法合并。
 */
export const PRESET_TRAFFIC_TEMPLATES: TrafficTemplate[] = [
    {
        id: 'preset-traffic-steady',
        name: '平稳 10 QPS',
        description: '5 分钟平稳 10 QPS 的基准流量',
        shapeType: 'baseline',
        controlPoints: [
            { x: 0, y: 10 },
            { x: 300, y: 10 },
        ],
        createdAt: '2026-01-01T00:00:00.000Z',
        updatedAt: '2026-01-01T00:00:00.000Z',
    },
    {
        id: 'preset-traffic-spike',
        name: '脉冲峰值',
        description: '前 2 分钟无流量，随后 1 分钟冲到 50 QPS 并维持，再回落',
        shapeType: 'spike',
        controlPoints: [
            { x: 0, y: 0 },
            { x: 60, y: 0 },
            { x: 120, y: 50 },
            { x: 180, y: 50 },
            { x: 240, y: 0 },
            { x: 300, y: 0 },
        ],
        createdAt: '2026-01-01T00:00:00.000Z',
        updatedAt: '2026-01-01T00:00:00.000Z',
    },
    {
        id: 'preset-traffic-ramp',
        name: '渐进斜坡',
        description: '5 分钟从 0 线性爬坡到 25 QPS',
        shapeType: 'custom',
        controlPoints: [
            { x: 0, y: 0 },
            { x: 60, y: 5 },
            { x: 120, y: 10 },
            { x: 180, y: 15 },
            { x: 240, y: 20 },
            { x: 300, y: 25 },
        ],
        createdAt: '2026-01-01T00:00:00.000Z',
        updatedAt: '2026-01-01T00:00:00.000Z',
    },
]