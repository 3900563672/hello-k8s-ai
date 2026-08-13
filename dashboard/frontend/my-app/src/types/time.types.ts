export type SnapshotTrigger = 'time' | 'event'
export type SnapshotDomain = 'scheduler' | 'configuration' | 'capacity' | 'runtime'
export type SnapshotSeverity = 'normal' | 'attention' | 'critical'

export interface SnapshotImpact {
    tenants: number
    nodes: number
    models: number
    changes: number
}

export interface Snapshot {
    id: string
    timestamp: string
    weight: number
    type: 'config' | 'event'
    trigger: SnapshotTrigger
    domain: SnapshotDomain
    severity: SnapshotSeverity
    title: string
    summary: string
    source: string
    impact: SnapshotImpact
    tags: string[]
}
