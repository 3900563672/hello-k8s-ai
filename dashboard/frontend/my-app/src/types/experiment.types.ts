import type { ApiEnvelope } from '@/types/api.types'
import type { TraceSummary } from '@/types/trace.types'

export type ExperimentStatus = 'pending' | 'running' | 'completed' | 'failed'

export interface ExperimentRecord {
    segmentId: string
    tenant: string
    name: string
    status: ExperimentStatus
    reason?: string
    configSnapshot?: unknown
    startSnapshot?: unknown
    endSnapshot?: unknown
    summary?: ExperimentSummary
    startedAt?: string
    endedAt?: string
    createdAt: string
    updatedAt: string
}

export interface ExperimentSummary {
    durationSeconds: number
    eventCounts: Record<string, number>
    metricBuckets: number
}

export interface SegmentEventRecord {
    eventId: string
    segmentId: string
    eventType: string
    occurredAt: string
    entity?: string
    severity?: string
    payload?: unknown
}

export interface MetricBucketRecord {
    metricName: string
    bucketStart: string
    bucketEnd: string
    min: number
    max: number
    avg: number
    p95: number
}

export interface ExperimentDetail {
    segment: ExperimentRecord
    events: SegmentEventRecord[]
    metrics: MetricBucketRecord[]
    traces: TraceSummary[]
}

export type ExperimentListEnvelope = ApiEnvelope<ExperimentRecord[]>
export type ExperimentDetailEnvelope = ApiEnvelope<ExperimentDetail>