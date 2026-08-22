import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MiniTimeline } from '@/components/shared/TimeTravelBar/MiniTimeline'

vi.mock('@/components/shared/TimeTravelBar/TimelineChart', () => ({
    TimelineChart: ({ variant }: { variant: string }) => (
        <div data-testid="timeline-chart-mock">{variant}</div>
    ),
}))

describe('MiniTimeline', () => {
    it('渲染高密度迷你时间线（mini 变体）', () => {
        render(<MiniTimeline />)
        expect(screen.getByTestId('timeline-chart-mock')).toHaveTextContent('mini')
    })
})