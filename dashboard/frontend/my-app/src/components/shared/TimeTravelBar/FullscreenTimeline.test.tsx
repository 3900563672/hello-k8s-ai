import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { FullscreenTimeline } from '@/components/shared/TimeTravelBar/FullscreenTimeline'
import { useTimeStore } from '@/stores/timeSlice'
import type { Snapshot } from '@/types/time.types'

vi.mock('@/components/shared/TimeTravelBar/TimelineChart', () => ({
    TimelineChart: () => <div data-testid="timeline-chart" />,
}))

const makeSnapshot = (id: string, timestamp: string, title = id): Snapshot => ({
    id,
    timestamp,
    weight: 1,
    type: 'event',
    trigger: 'event',
    domain: 'scheduler',
    severity: 'attention',
    title,
    summary: '摘要-' + title,
    source: 'test',
    impact: { tenants: 2, nodes: 3, models: 1, changes: 4 },
    tags: ['tag-1', 'tag-2'],
})

describe('FullscreenTimeline', () => {
    const onOpenChange = vi.fn()

    beforeEach(() => {
        onOpenChange.mockClear()
        useTimeStore.setState({
            mode: 'latest',
            timestamp: '2026-01-01T00:00:00.000Z',
            selectedSnapshotId: null,
            revision: 0,
            snapshots: [],
        })
    })

    it('打开时渲染探索器标题、图表与范围预设', () => {
        render(<FullscreenTimeline open onOpenChange={onOpenChange} />)
        expect(screen.getByText('时间切面探索器')).toBeInTheDocument()
        expect(screen.getByTestId('timeline-chart')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /1 分钟/ })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /全部/ })).toBeInTheDocument()
    })

    it('空快照时展示无可回放切面提示', () => {
        render(<FullscreenTimeline open onOpenChange={onOpenChange} />)
        expect(screen.getByText('暂无可回放切面')).toBeInTheDocument()
    })

    it('有快照且选中时展示详情与回放锚点', () => {
        useTimeStore.setState({
            timestamp: '2026-01-01T00:00:00.000Z',
            snapshots: [
                makeSnapshot('snap-1', '2026-01-01T00:00:00.000Z', '调度器变更'),
                makeSnapshot('snap-2', '2026-01-01T01:00:00.000Z', '容量调整'),
            ],
            selectedSnapshotId: 'snap-1',
        })
        render(<FullscreenTimeline open onOpenChange={onOpenChange} />)
        expect(screen.getByText('全站回放锚点')).toBeInTheDocument()
        expect(screen.getByText('snap-1')).toBeInTheDocument()
        expect(screen.getByText('调度器变更')).toBeInTheDocument()
    })

    it('无效时间跳转提示错误', async () => {
        const user = userEvent.setup()
        render(<FullscreenTimeline open onOpenChange={onOpenChange} />)
        const input = screen.getByLabelText('UTC 精确跳转时间')
        await user.clear(input)
        await user.type(input, 'not-a-time')
        await user.click(screen.getByRole('button', { name: /定位并回放/ }))
        expect(screen.getByText('请输入有效的 UTC 时间')).toBeInTheDocument()
    })
})
