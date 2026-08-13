export interface ApiEnvelope<T> {
    data: T
    meta: ApiMeta
}

export interface ApiMeta {
    requestId: string
    servedAt: string
    partial: boolean
    warnings: string[]
    sourceVersions: Record<string, string>
}

export interface ApiProblem {
    code: string
    message: string
    retryable: boolean
    details?: Record<string, unknown>
}

export interface ApiProblemEnvelope {
    error: ApiProblem
    meta?: Partial<ApiMeta>
}

export interface PageInfo {
    cursor: string | null
    hasMore: boolean
}

export interface Paginated<T> {
    items: T[]
    page: PageInfo
}
