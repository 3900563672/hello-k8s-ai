import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useBackendSync } from '@/hooks/useBackendSync'
import { useTimeStore } from '@/stores/timeSlice'
import { useControlPlaneStore } from '@/stores/controlPlaneSlice'
import { resetReplayStore } from '@/test/queryUtils'
import bootstrapFixture from '@/lib/mocks/fixtures/bootstrap.json'

class MockEventSource {
    static instances: MockEventSource[] = []
    url: string
    closed = false
    private listeners = new Map<string, Set<(event: Event) => void>>()

    constructor(url: string) {
        this.url = url
        MockEventSource.instances.push(this)
    }

    addEventListener(type: string, handler: (event: Event) => void) {
        if (!this.listeners.has(type)) this.listeners.set(type, new Set())
        this.listeners.get(type)!.add(handler)
    }

    removeEventListener(type: string, handler: (event: Event) => void) {
        this.listeners.get(type)?.delete(handler)
    }

    emit(type: string) {
        this.listeners.get(type)?.forEach((handler) => handler(new Event(type)))
    }

    close() {
        this.closed = true
    }
}

const okResponse = (body: unknown): Response =>
    new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } })

const meta = { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} }

const timelineSnapshot = (id: string, timestamp: string) => ({
    id,
    timestamp,
    label: id,
    source: 'postgresql-snapshot',
    impact: { tenants: 0, nodes: 0, models: 0, changes: 0 },
    tags: [],
})

describe('useBackendSync', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
        resetReplayStore()
        useControlPlaneStore.setState(useControlPlaneStore.getInitialState())
        MockEventSource.instances = []
        vi.stubGlobal('EventSource', MockEventSource)
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
        vi.unstubAllGlobals()
        vi.useRealTimers()
    })

    it('挂载即同步：拉取 bootstrap 与时间线并写入 snapshots', async () => {
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
            const url = String(input)
            if (url.includes('/replay')) {
                return okResponse({ data: { timeline: [timelineSnapshot('snap-1', '2026-08-20T00:00:00.000Z')] }, meta })
            }
            return okResponse(bootstrapFixture)
        })
        renderHook(() => useBackendSync())
        await waitFor(() => expect(useTimeStore.getState().snapshots).toHaveLength(1))
        expect(useTimeStore.getState().snapshots[0].id).toBe('snap-1')
        expect(useControlPlaneStore.getState().refreshPhase).toBe('success')
        const stream = MockEventSource.instances[0]
        expect(stream.url).toContain('/stream?topics=resources,events,clock')
    })

    it('时间线为空时回退到集群 serverTime', async () => {
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
            const url = String(input)
            if (url.includes('/replay')) return okResponse({ data: { timeline: [] }, meta })
            return okResponse(bootstrapFixture)
        })
        renderHook(() => useBackendSync())
        await waitFor(() => expect(useControlPlaneStore.getState().refreshPhase).toBe('success'))
        await waitFor(() => expect(useTimeStore.getState().timestamp).not.toBe(new Date(0).toISOString()))
    })

    it('resource.changed 事件触发防抖后重新同步，卸载时关闭 EventSource', async () => {
        vi.useFakeTimers()
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
            const url = String(input)
            if (url.includes('/replay')) return okResponse({ data: { timeline: [] }, meta })
            return okResponse(bootstrapFixture)
        })
        const { unmount } = renderHook(() => useBackendSync())
        await act(async () => { await Promise.resolve() })
        const stream = MockEventSource.instances[0]
        const callsBefore = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls.length
        act(() => stream.emit('resource.changed'))
        await act(async () => { vi.advanceTimersByTime(400) })
        expect((globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls.length).toBeGreaterThan(callsBefore)
        unmount()
        expect(stream.closed).toBe(true)
    })
})