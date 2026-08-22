// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { API_BASE_URL, ApiRequestError, apiData, apiRequest } from '@/api/client'

const okResponse = (body: unknown, status = 200): Response =>
    new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
    })

describe('api client', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
        vi.useRealTimers()
    })

    it('apiData 解析统一 envelope 并返回 data', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({
            data: { tenants: [] },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }))
        const data = await apiData<{ tenants: unknown[] }>('/configuration')
        expect(data).toEqual({ tenants: [] })
        const [url, init] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0]
        expect(String(url)).toBe(`${API_BASE_URL}/configuration`)
        expect(new Headers(init?.headers).get('Accept')).toBe('application/json')
    })

    it('非 2xx 解析 problem envelope 并抛 ApiRequestError', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({
            error: { code: 'NOT_FOUND', message: '资源不存在' },
            meta: { requestId: 'r2', servedAt: '2026-08-20T00:00:00.000Z' },
        }, 404))
        const error = await apiRequest('/configuration/models/m').catch((caught: unknown) => caught)
        expect(error).toBeInstanceOf(ApiRequestError)
        if (error instanceof ApiRequestError) {
            expect(error.status).toBe(404)
            expect(error.message).toBe('资源不存在')
            expect(error.problem?.code).toBe('NOT_FOUND')
        }
    })

    it('无 problem body 时使用通用失败文案', async () => {
        globalThis.fetch = vi.fn(async () => okResponse('oops', 500))
        const error = await apiRequest('/configuration').catch((caught: unknown) => caught)
        expect(error).toBeInstanceOf(ApiRequestError)
        if (error instanceof ApiRequestError) {
            expect(error.status).toBe(500)
            expect(error.message).toBe('请求失败（500）')
            expect(error.problem).toBeNull()
        }
    })

    it('网络错误转为 status 0 的 ApiRequestError', async () => {
        globalThis.fetch = vi.fn(async () => {
            throw new TypeError('fetch failed')
        })
        const error = await apiRequest('/configuration').catch((caught: unknown) => caught)
        expect(error).toBeInstanceOf(ApiRequestError)
        if (error instanceof ApiRequestError) {
            expect(error.status).toBe(0)
            expect(error.message).toBe('fetch failed')
        }
    })

    it('15 秒超时抛出“请求超时”', async () => {
        vi.useFakeTimers()
        globalThis.fetch = vi.fn((_input: RequestInfo | URL, init?: RequestInit) =>
            new Promise<Response>((_resolve, reject) => {
                init?.signal?.addEventListener('abort', () => {
                    reject(new DOMException('Aborted', 'AbortError'))
                })
            }),
        )
        const pending = apiRequest('/configuration')
        vi.advanceTimersByTime(15_001)
        const error = await pending.catch((caught: unknown) => caught)
        expect(error).toBeInstanceOf(ApiRequestError)
        if (error instanceof ApiRequestError) {
            expect(error.status).toBe(0)
            expect(error.message).toBe('Dashboard Backend 请求超时')
        }
    })

    it('外部 signal 中止时保留原始错误信息', async () => {
        const controller = new AbortController()
        globalThis.fetch = vi.fn((_input: RequestInfo | URL, init?: RequestInit) =>
            new Promise<Response>((_resolve, reject) => {
                init?.signal?.addEventListener('abort', () => {
                    reject(new DOMException('This operation was aborted', 'AbortError'))
                })
            }),
        )
        const pending = apiRequest('/configuration', { signal: controller.signal })
        controller.abort()
        const error = await pending.catch((caught: unknown) => caught)
        expect(error).toBeInstanceOf(ApiRequestError)
        if (error instanceof ApiRequestError) {
            expect(error.status).toBe(0)
            expect(error.message).toContain('aborted')
        }
    })
})
