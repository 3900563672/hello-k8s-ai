export type TrafficViewMode = 'total' | 'single' | 'compare'
export type TrafficWorkspaceMode = 'overview' | 'draw'

export interface TrafficPoint {
    /** 从 T+0 开始计算的逻辑时间，单位为秒。 */
    x: number
    /** 该时间点的绝对 QPS 增量。 */
    y: number
}

export interface TenantInfo {
    id: string
    name: string
    priority: 'P1' | 'P2' | 'P3' | 'P4' | 'P5'
    requestedQPS?: number
    allocatedQPS?: number
    runtimePhase?: string
}

/** 读取模式下的租户目标 QPS 序列（当前控制面为常量，故为单点）。 */
export interface FutureTrafficData {
    tenantId: string
    tenantName: string
    timeSeconds: number[]
    values: number[]
}

/** 应用流量叠加后 Backend 返回的写入回执。 */
export interface TrafficApplyReceipt {
    tenantId: string
    qps: number
    resourceVersion?: string
    convergence: string
}

export type TrafficTemplateShape =
    | 'spike'
    | 'step'
    | 'sine'
    | 'baseline'
    | 'custom'

export interface TrafficTemplate {
    id: string
    name: string
    description?: string
    /** 为兼容旧数据保留；实际曲线始终以 controlPoints 为准。 */
    shapeType: TrafficTemplateShape
    /** 真实坐标：x 为秒，y 为 QPS，不做归一化。 */
    controlPoints: TrafficPoint[]
    createdAt: string
    updatedAt: string
}

export interface OverlayInstance {
    id: string
    templateId: string
    templateName: string
    tenantId: string
    tenantName: string
    /** 相对 T+0 的逻辑开始时间。 */
    startOffsetSeconds: number
    /** 作为场景基线的全局回放上下文。 */
    effectiveAt: string
    snapshotId: string | null
    enabled: boolean
    color: string
    createdAt: string
}

export interface PreviewField {
    key: string
    value: string | number
    unit?: string
}

export type PreviewConfig = PreviewField[]
