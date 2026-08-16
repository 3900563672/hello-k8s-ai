export const TENANT_PRIORITIES = ['P1', 'P2', 'P3', 'P4', 'P5'] as const

export type TenantPriority = (typeof TENANT_PRIORITIES)[number]
export type ConfigResourceType = 'model' | 'node' | 'tenant' | 'orchestrator'

export interface ModelPerformance {
    prefillBaseMs: number
    prefillPerTokenUs: number
    decodePerTokenMs: number
}

export interface ModelSpec {
    displayName: string
    gpuUnits: number
    maxConcurrency: number
    absoluteScore: number
    coldStartMs: number
    performance: ModelPerformance
}

export interface NodeSpec {
    displayName: string
    gpu: number
    maxConcurrency: number
}

export interface TenantSpec {
    displayName: string
    priority: TenantPriority
    qps: number
    ttftThresholdMs: number
    queueThreshold: number
    ttftScaleDownThresholdMs: number
    queueScaleDownThreshold: number
}

export interface OrchestratorSpec {
    tenantRef: { name: string }
    scaleUpCooldownSeconds: number
    scaleDownCooldownSeconds: number
    allowScaleToZero: boolean
    minReplicas: number
    maxReplicas: number
}

export interface Model extends ModelSpec {
    name: string
    uid?: string
    resourceVersion?: string
    status?: Record<string, unknown>
    conditions?: KubernetesCondition[]
    derived?: Record<string, unknown>
}

export interface Node extends NodeSpec {
    name: string
    uid?: string
    resourceVersion?: string
    status?: Record<string, unknown>
    conditions?: KubernetesCondition[]
    derived?: Record<string, unknown>
}

export interface Tenant extends TenantSpec {
    name: string
    uid?: string
    resourceVersion?: string
    status?: Record<string, unknown>
    conditions?: KubernetesCondition[]
    derived?: Record<string, unknown>
}

export interface Orchestrator extends OrchestratorSpec {
    name: string
    // Orchestrator 没有 displayName 字段，这里展示关联租户名以满足通用列表约束
    displayName: string
    uid?: string
    resourceVersion?: string
    status?: Record<string, unknown>
    conditions?: KubernetesCondition[]
    derived?: Record<string, unknown>
}

export const POLICY_EFFECTS = ['Allow', 'Deny'] as const

export type PolicyEffect = (typeof POLICY_EFFECTS)[number]

export type PolicyKind = 'tenantModel' | 'tenantNode' | 'modelNode'

export interface PolicySpec {
    tenantRef?: { name: string }
    modelRef?: { name: string }
    nodeRef?: { name: string }
    effect: PolicyEffect
}

export interface Policy {
    name: string
    displayName: string
    kind: PolicyKind
    tenantRef?: { name: string }
    modelRef?: { name: string }
    nodeRef?: { name: string }
    effect: PolicyEffect
    uid?: string
    resourceVersion?: string
    status?: Record<string, unknown>
    conditions?: KubernetesCondition[]
    derived?: Record<string, unknown>
}

export interface KubernetesCondition {
    type: string
    status: string
    reason?: string
    message?: string
    observedGeneration?: number
    lastTransitionTime?: string
}

export interface BackendResource<TSpec extends Record<string, unknown> = Record<string, unknown>> {
    ref: {
        apiVersion: string
        kind: string
        name: string
        uid?: string
    }
    metadata: {
        generation: number
        resourceVersion: string
        createdAt?: string
    }
    spec: TSpec
    status: Record<string, unknown>
    conditions: KubernetesCondition[]
    derived?: Record<string, unknown>
}

export interface ConfigurationReadModel {
    asOf: string
    availability: 'available' | 'unavailable'
    models: BackendResource<ModelSpec & Record<string, unknown>>[]
    workerNodes: BackendResource<NodeSpec & Record<string, unknown>>[]
    tenants: BackendResource<TenantSpec & Record<string, unknown>>[]
    policies: {
        tenantModel: BackendResource[]
        tenantNode: BackendResource[]
        modelNode: BackendResource[]
    }
    orchestrators: BackendResource<OrchestratorSpec & Record<string, unknown>>[]
    simulationClocks: BackendResource[]
    simulatorInstances: BackendResource[]
    tenantPerformance: BackendResource[]
    tenantRuntimes: BackendResource[]
}

export type ConfigResource = Model | Node | Tenant

export interface PreviewField {
    key: string
    value: string | number
    unit?: string
}

export type PreviewConfig = PreviewField[]

export interface ConfigTemplate<T> {
    id: string
    name: string
    data: T
    createdAt: string
    /** 预置模板为内置样例，只存在于内存；用户保存的模板不设置该字段。 */
    preset?: boolean
}
