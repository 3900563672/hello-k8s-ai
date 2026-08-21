import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ExperimentPanel } from '@/components/features/trace/ExperimentPanel'
import type { ExperimentDetail, ExperimentRecord } from '@/types/experiment.types'

const records: ExperimentRecord[] = [
    {
        segmentId: 'seg-1',
        tenant: 'tenant-a',
        name: '扩容验证-20x',
        status: 'running',
        createdAt: '2026-08-21T00:00:00.000Z',
        updatedAt: '2026-08-21T00:01:30.000Z',
        startedAt: '2026-08-21T00:01:00.000Z',
    },
    {
        segmentId: 'seg-2',
        tenant: 'tenant-b',
        name: '缩容验证',
        status: 'completed',
        updatedAt: '2026-08-21T00:03:00.000Z',
        createdAt: '2026-08-20T00:00:00.000Z',
    },
]

const detail: ExperimentDetail = {
    segment: records[0],
    events: [
        { eventId: 'evt-1', segmentId: 'seg-1', eventType: 'decision', occurredAt: '2026-08-21T00:02:00.000Z', entity: 'model-a', severity: 'info' },
        { eventId: 'evt-2', segmentId: 'seg-1', eventType: 'alert', occurredAt: '2026-08-21T00:03:00.000Z', entity: 'node-a', severity: 'warning' },
    ],
    metrics: [],
    traces: [],
}

const mutate = vi.fn()
const startMutate = vi.fn()
const completeMutate = vi.fn()
const failMutate = vi.fn()

vi.mock('@/api/queries/experimentQueries', () => ({
    useExperiments: () => ({ data: { data: records } }),
    useExperimentDetail: (segmentId: string | null) => ({
        data: segmentId ? { data: detail } : undefined,
        isPending: false,
        isError: false,
        error: null,
    }),
    useCreateExperiment: () => ({ mutate }),
    useStartExperiment: () => ({ mutate: startMutate, isPending: false }),
    useCompleteExperiment: () => ({ mutate: completeMutate, isPending: false }),
    useFailExperiment: () => ({ mutate: failMutate, isPending: false }),
}))

describe('ExperimentPanel', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    it('渲染实验列表与状态徽标，自动选中运行中实验', () => {
        render(<ExperimentPanel />)
        expect(screen.getByText('实验切面（Experiment）')).toBeInTheDocument()
        expect(screen.getByText('扩容验证-20x')).toBeInTheDocument()
        expect(screen.getByText('运行中')).toBeInTheDocument()
        expect(screen.getByText('已完成')).toBeInTheDocument()
        expect(screen.getByText('采样中')).toBeInTheDocument()
    })

    it('展示运行中实验的事件序列', () => {
        render(<ExperimentPanel />)
        expect(screen.getByText('事件序列')).toBeInTheDocument()
        expect(screen.getByText('决策')).toBeInTheDocument()
        expect(screen.getByText('告警')).toBeInTheDocument()
        expect(screen.getByText('model-a')).toBeInTheDocument()
    })

    it('点击完成按钮调用完成接口', async () => {
        const user = userEvent.setup()
        render(<ExperimentPanel />)
        await user.click(screen.getByRole('button', { name: /完成/ }))
        expect(completeMutate).toHaveBeenCalledWith('seg-1')
    })

    it('点击失败展开原因输入并确认失败', async () => {
        const user = userEvent.setup()
        render(<ExperimentPanel />)
        await user.click(screen.getByRole('button', { name: /^失败$/ }))
        await user.type(screen.getByPlaceholderText('失败原因…'), 'QPS 超限')
        await user.click(screen.getByRole('button', { name: /确认失败/ }))
        expect(failMutate).toHaveBeenCalledWith({ segmentId: 'seg-1', reason: 'QPS 超限' })
    })

    it('已完成实验展示查看按钮并切换到详情', async () => {
        const user = userEvent.setup()
        render(<ExperimentPanel />)
        await user.click(screen.getByText('缩容验证'))
        expect(screen.queryByText('采样中')).not.toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: /查看/ }))
        expect(screen.getByText('事件序列')).toBeInTheDocument()
    })

    it('输入名称创建实验', async () => {
        const user = userEvent.setup()
        const envelope = { data: { segment: { segmentId: 'seg-3' } } }
        mutate.mockImplementation((_payload, handlers) => {
            handlers?.onSuccess?.(envelope)
        })
        render(<ExperimentPanel />)
        await user.type(screen.getByPlaceholderText('实验名称，例如：扩容验证-20x'), '新实验')
        await user.click(screen.getByRole('button', { name: /创建实验/ }))
        expect(mutate).toHaveBeenCalledWith(
            { tenant: 'default', name: '新实验' },
            expect.anything(),
        )
    })
})