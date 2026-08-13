import type { ApiEnvelope, ApiProblem, ApiProblemEnvelope } from '@/types/api.types'

const configuredBase = import.meta.env.VITE_API_BASE_URL?.trim()
export const API_BASE_URL = (configuredBase || '/api/v1').replace(/\/$/, '')
const DEFAULT_TIMEOUT_MS = 15_000

export class ApiRequestError extends Error {
    readonly status: number
    readonly problem: ApiProblem | null

    constructor(status: number, message: string, problem: ApiProblem | null = null) {
        super(message)
        this.name = 'ApiRequestError'
        this.status = status
        this.problem = problem
    }
}

export async function apiRequest<T>(
    path: string,
    init?: RequestInit,
): Promise<T> {
    const timeout = new AbortController()
    const timer = globalThis.setTimeout(() => timeout.abort(), DEFAULT_TIMEOUT_MS)
    const signal = init?.signal
        ? AbortSignal.any([init.signal, timeout.signal])
        : timeout.signal

    let response: Response
    try {
        response = await fetch(`${API_BASE_URL}${path}`, {
            ...init,
            signal,
            headers: {
                Accept: 'application/json',
                ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
                ...init?.headers,
            },
        })
    } catch (error) {
        if (timeout.signal.aborted) {
            throw new ApiRequestError(0, 'Dashboard Backend 请求超时')
        }
        throw new ApiRequestError(
            0,
            error instanceof Error ? error.message : '无法连接 Dashboard Backend',
        )
    } finally {
        globalThis.clearTimeout(timer)
    }

    if (!response.ok) {
        const body = (await response.json().catch(() => null)) as ApiProblemEnvelope | null
        const problem = body?.error ?? null
        throw new ApiRequestError(
            response.status,
            problem?.message ?? `请求失败（${response.status}）`,
            problem,
        )
    }

    return response.json() as Promise<T>
}

export async function apiData<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await apiRequest<ApiEnvelope<T>>(path, init)
    return response.data
}
