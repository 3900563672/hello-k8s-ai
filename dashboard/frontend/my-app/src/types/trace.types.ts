import type { ApiEnvelope } from '@/types/api.types'
import type { BackendResource, ConfigurationReadModel, KubernetesCondition } from '@/types/config.types'
import type { ReplayTimeContext } from '@/stores/timeSlice'
import type { AgentVerdict } from '@/types/aiops.types'

export interface OverviewQuery extends ReplayTimeContext {
    tenantId?: string
    modelId?: string
    instanceId?: string
}

export interface SegmentQuery {
    start: string
    end: string
    tenantId?: string
    modelId?: string
    instanceId?: string
}

export interface NumberValue {
    value: number
    unit: string
}

export interface BackendPod {
    ref: ResourceRef
    phase: string
    ready: boolean
    nodeName?: string
    podIP?: string
    startTime?: string
    conditions: KubernetesCondition[]
    containers: Array<{
        name: string
        ready: boolean
        restartCount: number
        state: string
        reason?: string
    }>
    simulatorInstance?: string
    tenant?: string
    model?: string
    /** AIOps L1 结论（外圈分级着色），未分析时不存在。 */
    agentVerdict?: AgentVerdict
}

export interface BackendNode {
    ref: ResourceRef
    role: string
    ready: boolean
    phase: string
    schedulable: boolean
    zone?: string
    version?: string
    internalIP?: string
    conditions: KubernetesCondition[]
    observedAt: string
    /** AIOps L1 结论（外圈分级着色），未分析时不存在。 */
    agentVerdict?: AgentVerdict
}

export interface BackendDeployment {
    ref: ResourceRef
    desiredReplicas: number
    updatedReplicas: number
    readyReplicas: number
    availableReplicas: number
    unavailableReplicas: number
    conditions: KubernetesCondition[]
    simulatorInstance?: string
    tenant?: string
    model?: string
}

export interface BackendEvent {
    id: string
    eventTime: string
    type: string
    reason: string
    message: string
    count: number
    regarding: ResourceRef
    reportingController?: string
}

export interface BackendLease {
    ref: ResourceRef
    holderIdentity?: string
    renewTime?: string
    leaseDurationSeconds?: number
    fresh: boolean
    ageMs?: number
}

export interface ResourceRef {
    apiVersion: string
    kind: string
    namespace?: string
    name: string
    uid?: string
}

export interface TrafficInstance {
    name: string
    model: string
    desiredReplicas: number
    availableReplicas: number
    assignedQPS: number
    score?: number
    effectiveScore?: number
    phase?: string
    observedAt?: string
    freshness: string
    pods: BackendPod[]
}

export interface TenantTraffic {
    tenant: ResourceRef
    displayName: string
    priority: string
    requestedQPS: number
    allocatedQPS: number
    allocationBalanced: boolean
    performance: {
        avgTTFT?: NumberValue
        avgQueue?: NumberValue
        sampleCount: number
        observedAt?: string
        freshness: string
        phase?: string
    }
    readyReplicaCount: number
    runtimePhase?: string
    instances: TrafficInstance[]
    /** AIOps L1 结论（外圈分级着色），未分析时不存在。 */
    agentVerdict?: AgentVerdict
}

export interface MetricPoint {
    time: string
    value: number
}

export interface MetricResult {
    metricId: string
    unit: string
    start: string
    end: string
    stepSeconds: number
    series: Array<{ labels: Record<string, string>; points: MetricPoint[] }>
    resultType: string
    warnings: string[]
    queriedAt: string
}

export interface TraceSummary {
    traceId: string
    rootService: string
    rootOperation: string
    startTime: string
    durationMs: number
    spanCount: number
    errorSpanCount: number
    entities: Record<string, string>
}

export interface TraceSpan {
    spanId: string
    parentSpanId?: string
    service: string
    operation: string
    startTime: string
    durationMs: number
    status: string
    attributes: Record<string, unknown>
    events: Array<{
        name: string
        time: string
        attributes?: Record<string, unknown>
    }>
}

export interface TraceDetail {
    traceId: string
    spans: TraceSpan[]
    entityLinks: ResourceRef[]
}

export interface ProviderState {
    state: string
    observedAt: string
    error?: string
    retention?: string
    storage?: string
}

export interface OverviewData {
    availability: 'available' | 'unavailable'
    asOf: string
    snapshotId?: string
    clock: {
        serverTime: string
        actualTime: string
        logicalTime: string
        rate: number
        state: string
        authoritative: boolean
        capabilities: {
            simulatorAcceleration: boolean
        }
    }
    configuration: ConfigurationReadModel
    traffic: { asOf: string; tenants: TenantTraffic[] }
    workloads: {
        nodes: BackendNode[]
        pods: BackendPod[]
        deployments: BackendDeployment[]
        services: Array<{ ref: ResourceRef; type: string; clusterIP?: string }>
        leases: BackendLease[]
        events: BackendEvent[]
    }
    metrics: Record<string, MetricResult>
    traces: TraceSummary[]
    freshness: Record<string, ProviderState>
}

export interface SegmentSnapshotData {
    snapshotId?: string
    capturedAt: string
    configuration: ConfigurationReadModel
    traffic: { asOf: string; tenants: TenantTraffic[] }
    workloads: {
        nodes: BackendNode[]
        pods: BackendPod[]
        deployments: BackendDeployment[]
        services: Array<{ ref: ResourceRef; type: string; clusterIP?: string }>
        leases: BackendLease[]
        events: BackendEvent[]
    }
}

export interface SegmentOverviewData {
    availability: 'available' | 'unavailable'
    start: string
    end: string
    startSnapshot?: SegmentSnapshotData
    endSnapshot?: SegmentSnapshotData
    metrics: Record<string, MetricResult>
    traces: TraceSummary[]
    freshness: Record<string, ProviderState>
}

export type OverviewEnvelope = ApiEnvelope<OverviewData>
export type SegmentEnvelope = ApiEnvelope<SegmentOverviewData>
export type TraceDetailEnvelope = ApiEnvelope<TraceDetail>
export type AnyPlatformResource = BackendResource<Record<string, unknown>>
