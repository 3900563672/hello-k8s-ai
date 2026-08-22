import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useTimeStore } from '@/stores/timeSlice'
import { useControlPlaneStore } from '@/stores/controlPlaneSlice'
import { TimeTravelBar } from '@/components/shared/TimeTravelBar/TimeTravelBar'
import { resetReplayStore } from '@/test/queryUtils'
import type { Snapshot } from '@/types/time.types'

vi.mock('@/components/shared/TimeTravelBar/FullscreenTimeline', () => ({
    FullscreenTimeline: () => <div data-testid="fullscreen-timeline" />,
}))
vi.mock('@/components/shared/TimeTravelBar/MiniTimeline', () => ({
    MiniTimeline: () => <div data-testid="mini-timeline" />,
}))

const makeSnapshot = (id: string, timestamp: string): Snapshot => ({
    id,
    timestamp,
    weight: 0,
    type: 'config',
    trigger: 'time',
    domain: 'scheduler',
    severity: 'normal',
    title: `snapshot-${id}`,
    summary: '',
    source: 'postgresql-snapshot',
    impact: { tenants: 0, nodes: 0, models: 0, changes: 0 },
    tags: [],
})

const setLatest = (): void => {
    useTimeStore.setState({
        timestamp: '2026-08-20T01:00:00.000Z',
        mode: 'latest',
        selectedSnapshotId: null,
        revision: 2,
        snapshots: [
            makeSnapshot('snap-1', '2026-08-20T00:00:00.000Z'),
            makeSnapshot('snap-2', '2026-08-20T01:00:00.000Z'),
        ],
    })
}

const setHistorical = (): void => {
    useTimeStore.setState({
        timestamp: '2026-08-20T00:30:00.000Z',
        mode: 'historical',
        selectedSnapshotId: 'snap-1',
        revision: 2,
        snapshots: [
            makeSnapshot('snap-1', '2026-08-20T00:00:00.000Z'),
            makeSnapshot('snap-2', '2026-08-20T01:00:00.000Z'),
        ],
    })
}

describe('TimeTravelBar', () => {
    beforeEach(() => {
        resetReplayStore()
        useControlPlaneStore.setState({ cluster: useControlPlaneStore.getInitialState().cluster })
    })
    afterEach(() => {
        useControlPlaneStore.setState(useControlPlaneStore.getInitialState())
    })

    it('latest 模式展示最新徽章与权威时间', () => {
        setLatest()
        render(<TimeTravelBar />)
        expect(screen.getByText('最新')).toBeInTheDocument()
        expect(screen.getByText('2026-08-20 01:00:00.000')).toBeInTheDocument()
        expect(screen.getByText('UTC')).toBeInTheDocument()
        expect(screen.getByTestId('mini-timeline')).toBeInTheDocument()
    })

    it('historical 模式展示历史徽章、切面步进与回到最新', async () => {
        const user = userEvent.setup()
        setHistorical()
        render(<TimeTravelBar />)
        expect(screen.getByText('历史')).toBeInTheDocument()

        // 第一个切面：上一切面禁用；回到最新按钮存在（xl 断点下可见）
        expect(screen.getByRole('button', { name: '上一切面' })).toBeDisabled()
        expect(document.querySelector('button[title="回到最新切面"]')).not.toBeNull()

        // 步进到最后一个切面：选中最后一条即回到 latest，下一切面禁用
        await user.click(screen.getByRole('button', { name: '下一切面' }))
        expect(useTimeStore.getState().selectedSnapshotId).toBe('snap-2')
        expect(useTimeStore.getState().mode).toBe('latest')
        expect(screen.getByRole('button', { name: '下一切面' })).toBeDisabled()
        expect(document.querySelector('button[title="回到最新切面"]')).toBeNull()
    })

    it('等待 Backend 权威时间时展示占位文案', () => {
        resetReplayStore()
        render(<TimeTravelBar />)
        expect(screen.getByText('等待 Backend 权威时间')).toBeInTheDocument()
    })
})