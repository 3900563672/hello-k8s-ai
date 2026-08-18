import type { Model, Node, Orchestrator, Tenant } from '@/types/config.types'

export const DEFAULT_MODEL: Omit<Model, 'name' | 'displayName'> = {
    gpuUnits: 1,
    maxConcurrency: 1,
    absoluteScore: 100,
    coldStartMs: 0,
    // 与 CRD 字段级默认值保持一致，保证新建资源无需编辑即可通过校验
    performance: {
        prefillBaseMs: 50,
        prefillPerTokenUs: 500,
        decodePerTokenMs: 20,
    },
}

export const DEFAULT_NODE: Omit<Node, 'name' | 'displayName'> = {
    gpu: 1,
    maxConcurrency: 1,
}

export const DEFAULT_TENANT: Omit<Tenant, 'name' | 'displayName'> = {
    priority: 'P3',
    qps: 0,
    // 阈值必填且必须为正数；初始值与 CRD 默认值一致，新建租户无需编辑即可通过校验
    ttftThresholdMs: 500,
    queueThreshold: 100,
    ttftScaleDownThresholdMs: 200,
    queueScaleDownThreshold: 30,
}

export const DEFAULT_ORCHESTRATOR: Omit<Orchestrator, 'name' | 'displayName' | 'tenantRef'> = {
    scaleUpCooldownSeconds: 60,
    scaleDownCooldownSeconds: 120,
    allowScaleToZero: false,
    minReplicas: 1,
    maxReplicas: 0, // 0 = 无限制（模拟器无网关，接受任意 QPS，扩到容量上限为止）
    maxScaleUpBatch: 10, // 0 = 使用默认 10（每轮扩容最多补的副本数）
}
