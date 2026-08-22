import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useControlPlaneStore } from '@/stores/controlPlaneSlice'
import { useTimeStore } from '@/stores/timeSlice'
import { ClusterStatus } from '@/components/shared/Layout/ClusterStatus'
import { resetReplayStore } from '@/test/queryUtils'
import type { ClusterSnapshot } from '@/types/control-plane.types'

const makeCluster = (overrides: Partial<ClusterSnapshot> = {}): ClusterSnapshot => ({
    ...useControlPlaneStore.getInitialState().cluster,
    connectionStatus: 'connected',
    simulationRunSupported: true,
    simulationRateSupported: true,
    checkedAt: '2026-08-20T00:00:00.000Z',
    clockResourceVersion: '',
    clockConverged: false,
    controlPlane: {
        id: 'cp-0',
        name: 'control-plane-0',
        role: 'control-plane',
        status: 'running',
        ready: true,
        zone: 'zone-a',
        gpuCapacity: 0,
    },
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

const receipt = {
    id: 'd1',
    clusterId: 'c1',
    acceptedNodes: 1,
    resources: { models: 1, nodes: 1, tenants: 1, total: 3, revision: 'r1' },
    createdAt: '2026-08-20T00:00:00.000Z',
}

describe('ClusterStatus', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
        resetReplayStore()
        useControlPlaneStore.setState({ cluster: makeCluster() })
    })
    afterEach(() => {
        globalThis.fetch = originalFetch
        useControlPlaneStore.setState(useControlPlaneStore.getInitialState())
    })

    it('展示连接状态与在线节点计数', () => {
        render(<ClusterStatus />)
        expect(screen.getByText('1/1')).toBeInTheDocument()
        expect(screen.getByText('已连接')).toBeInTheDocument()
    })

    it('未连接时展示未连接文案', () => {
        useControlPlaneStore.setState({ cluster: makeCluster({ connectionStatus: 'disconnected' }) })
        render(<ClusterStatus />)
        expect(screen.getByText('未连接')).toBeInTheDocument()
    })

    it('打开详情浮层展示集群元信息与工作节点列表', async () => {
        const user = userEvent.setup()
        render(<ClusterStatus />)
        await user.click(screen.getByRole('button', { name: /打开集群详情/ }))
        expect(await screen.findByText('Worker nodes')).toBeInTheDocument()
        expect(screen.getByText('worker-0')).toBeInTheDocument()
        expect(screen.getByText('运行中')).toBeInTheDocument()
        expect(screen.getByText('已连接 · 只读')).toBeInTheDocument()
    })

    it('历史回放时展示只读提示', async () => {
        const user = userEvent.setup()
        useTimeStore.setState({ mode: 'historical', selectedSnapshotId: 's1', revision: 1 })
        render(<ClusterStatus />)
        await user.click(screen.getByRole('button', { name: /打开集群详情/ }))
        expect(await screen.findByText(/历史回放为只读/)).toBeInTheDocument()
        expect(screen.getByRole('button', { name: '核验配置' })).toBeDisabled()
    })

    it('分发成功后展示已核验反馈', async () => {
        const user = userEvent.setup()
        globalThis.fetch = vi.fn(async () => new Response(JSON.stringify({
            data: {
                models: [{ id: 'm1' }],
                workerNodes: [{ id: 'n1' }],
                tenants: [{ id: 't1' }],
            },
            meta: {
                requestId: 'r1',
                servedAt: '2026-08-20T00:00:00.000Z',
                partial: false,
                warnings: [],
                sourceVersions: { kubernetes: 'kv1' },
            },
        }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
        render(<ClusterStatus />)
        await user.click(screen.getByRole('button', { name: /打开集群详情/ }))
        await user.click(await screen.findByRole('button', { name: '核验配置' }))
        expect(await screen.findByText('已核验 3 项')).toBeInTheDocument()
    })

    it('分发成功反馈会在 1.8s 后自动清除', async () => {
        const user = userEvent.setup()
        useControlPlaneStore.setState({ distributionPhase: 'success', distributionReceipt: receipt })
        render(<ClusterStatus />)
        await user.click(screen.getByRole('button', { name: /打开集群详情/ }))
        expect(await screen.findByText('已核验 3 项')).toBeInTheDocument()
        await waitFor(
            () => expect(screen.queryByText('已核验 3 项')).not.toBeInTheDocument(),
            { timeout: 3_000 },
        )
    })
})