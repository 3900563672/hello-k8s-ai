import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useControlPlaneStore } from '@/stores/controlPlaneSlice'
import { useTimeStore } from '@/stores/timeSlice'
import { ExecutionControls } from '@/components/shared/Layout/ExecutionControls'
import { resetReplayStore } from '@/test/queryUtils'
import type { ClusterSnapshot } from '@/types/control-plane.types'

const makeCluster = (overrides: Partial<ClusterSnapshot> = {}): ClusterSnapshot => ({
    ...useControlPlaneStore.getInitialState().cluster,
    connectionStatus: 'connected',
    simulationRunSupported: true,
    simulationRateSupported: true,
    clockRate: 1,
    clockAppliedRate: 1,
    clockResourceVersion: '',
    clockConverged: true,
    clockSynchronizedInstances: 1,
    clockTotalInstances: 1,
    workers: [
        {
            id: 'worker-0',
            name: 'worker-0',
            role: 'worker',
            status: 'running',
            ready: true,
            zone: 'zone-a',
            gpuCapacity: 8,
        },
    ],
    ...overrides,
})

const renderControls = () =>
    render(
        <TooltipProvider>
            <ExecutionControls />
        </TooltipProvider>,
    )

describe('ExecutionControls', () => {
    beforeEach(() => {
        resetReplayStore()
        useControlPlaneStore.setState({ cluster: makeCluster(), executionMode: 'apply', executionPhase: 'standby' })
    })
    afterEach(() => {
        useControlPlaneStore.setState(useControlPlaneStore.getInitialState())
    })

    it('集群可用时展示待命中并可切换到测试模式', async () => {
        const user = userEvent.setup()
        renderControls()
        expect(screen.getByText('待命中')).toBeInTheDocument()

        await user.click(screen.getByRole('button', { name: '测试运行模式' }))
        expect(useControlPlaneStore.getState().executionMode).toBe('test')
        expect(useControlPlaneStore.getState().executionPhase).toBe('running')
        expect(screen.getByText('测试运行')).toBeInTheDocument()

        await user.click(screen.getByRole('button', { name: '应用模式' }))
        expect(useControlPlaneStore.getState().executionMode).toBe('apply')
    })

    it('无在线 Worker 时测试模式禁用', async () => {
        const user = userEvent.setup()
        useControlPlaneStore.setState({
            cluster: makeCluster({ workers: [{ ...makeCluster().workers[0], status: 'offline' }] }),
        })
        renderControls()
        const testButton = screen.getByRole('button', { name: '测试运行模式' })
        expect(testButton).toBeDisabled()
        await user.hover(testButton)
        expect(await screen.findByText('需要至少一个在线工作节点')).toBeInTheDocument()
    })
    it('历史回放时测试与倍速均禁用，状态为历史只读', () => {
        useTimeStore.setState({ mode: 'historical', selectedSnapshotId: 's1', revision: 1 })
        renderControls()
        expect(screen.getByRole('button', { name: '测试运行模式' })).toBeDisabled()
        expect(screen.getByRole('button', { name: '应用模式' })).toBeDisabled()
        expect(screen.getByLabelText('Simulator 时间倍速')).toBeDisabled()
        expect(screen.getByText('历史只读')).toBeInTheDocument()
    })

    it('执行错误时展示错误状态', () => {
        useControlPlaneStore.setState({ executionMode: 'apply', executionPhase: 'error' })
        renderControls()
        expect(screen.getByText('错误')).toBeInTheDocument()
    })

    it('倍速选择会调用 setSimulationRate', async () => {
        const user = userEvent.setup()
        const setSimulationRate = vi.spyOn(useControlPlaneStore.getState(), 'setSimulationRate').mockImplementation(() => Promise.resolve(true))
        renderControls()
        await user.selectOptions(screen.getByLabelText('Simulator 时间倍速'), '5')
        expect(setSimulationRate).toHaveBeenCalledWith(5)
        setSimulationRate.mockRestore()
    })
})