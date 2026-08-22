import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from '@testing-library/react'
import { TimelineChart } from '@/components/shared/TimeTravelBar/TimelineChart'
import { useTimeStore } from '@/stores/timeSlice'
import type { Snapshot } from '@/types/time.types'

const mocks = vi.hoisted(() => {
    const getZrHandlers: Record<string, (event: unknown) => void> = {}
    const chartHandlers: Record<string, (event: unknown) => void> = {}
    const chart = {
        on: vi.fn((name: string, handler: (event: unknown) => void) => {
            chartHandlers[name] = handler
        }),
        off: vi.fn(),
        resize: vi.fn(),
        dispose: vi.fn(),
        setOption: vi.fn(),
        containPixel: vi.fn(() => true),
        convertFromPixel: vi.fn(() => [100]),
        getZr: vi.fn(() => ({
            on: vi.fn((name: string, handler: (event: unknown) => void) => {
                getZrHandlers[name] = handler
            }),
            off: vi.fn(),
        })),
    }
    return { chart, getZrHandlers, chartHandlers }
})

vi.mock('echarts/core', () => ({
    use: vi.fn(),
    init: () => mocks.chart,
}))

class ResizeObserverStub {
    observe() {}
    unobserve() {}
    disconnect() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverStub)

const makeSnapshot = (id: string, timestamp: string): Snapshot => ({
    id,
    timestamp,
    weight: 1,
    type: 'event',
    trigger: 'event',
    domain: 'runtime',
    severity: 'normal',
    title: id,
    summary: '',
    source: 'test',
    impact: { tenants: 0, nodes: 0, models: 0, changes: 0 },
    tags: [],
})

describe('TimelineChart', () => {
    beforeEach(() => {
        mocks.chart.on.mockClear()
        mocks.chart.setOption.mockClear()
        useTimeStore.setState({
            mode: 'latest',
            timestamp: '2026-01-01T00:00:00.000Z',
            selectedSnapshotId: null,
            revision: 0,
            snapshots: [],
        })
    })

    it('空快照时初始化图表并渲染容器', () => {
        const { container } = render(<TimelineChart variant="mini" />)
        expect(mocks.chart.on).toHaveBeenCalledWith('datazoom', expect.any(Function))
        expect(container.querySelector('div')).not.toBeNull()
    })

    it('有快照时聚合桶并下发 setOption', () => {
        useTimeStore.setState({
            snapshots: [
                makeSnapshot('snap-1', '2026-01-01T00:00:00.000Z'),
                makeSnapshot('snap-2', '2026-01-01T01:00:00.000Z'),
            ],
        })
        render(<TimelineChart variant="explorer" />)
        expect(mocks.chart.setOption).toHaveBeenCalled()
    })

    it('datazoom 事件触发视口更新', () => {
        useTimeStore.setState({
            snapshots: [
                makeSnapshot('snap-1', '2026-01-01T00:00:00.000Z'),
                makeSnapshot('snap-2', '2026-01-01T01:00:00.000Z'),
            ],
        })
        render(<TimelineChart variant="mini" />)
        const handler = mocks.chartHandlers['datazoom']
        expect(handler).toBeDefined()
        // 视口会被 clampViewport 收敛到快照时间边界内，因此传入边界内的时间戳
        const boundsStart = Date.parse('2026-01-01T00:00:00.000Z')
        handler({
            batch: [{ startValue: boundsStart + 60_000, endValue: boundsStart + 600_000 }],
        })
        const viewport = useTimeStore.getState().viewport
        expect(viewport.start).toBe(boundsStart + 60_000)
        expect(viewport.end).toBe(boundsStart + 600_000)
    })

    it('画布点击跳转到对应时间戳', () => {
        render(<TimelineChart variant="mini" />)
        const handler = mocks.getZrHandlers['click']
        expect(handler).toBeDefined()
        handler({ offsetX: 100, offsetY: 50 })
        expect(useTimeStore.getState().timestamp).not.toBe('')
    })
})
