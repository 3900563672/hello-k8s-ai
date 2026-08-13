import type { Model, Node, Tenant } from '@/types/config.types'

export const DEFAULT_MODEL: Omit<Model, 'name' | 'displayName'> = {
    gpuUnits: 1,
    maxConcurrency: 1,
    coldStartMs: 0,
    performance: {
        prefillBaseMs: 0,
        prefillPerTokenUs: 0,
        decodePerTokenMs: 0,
    },
}

export const DEFAULT_NODE: Omit<Node, 'name' | 'displayName'> = {
    gpu: 1,
    maxConcurrency: 1,
}

export const DEFAULT_TENANT: Omit<Tenant, 'name' | 'displayName'> = {
    priority: 'P3',
    qps: 0,
    ttftThresholdMs: 0,
    queueThreshold: 0,
    ttftScaleDownThresholdMs: 0,
    queueScaleDownThreshold: 0,
}
