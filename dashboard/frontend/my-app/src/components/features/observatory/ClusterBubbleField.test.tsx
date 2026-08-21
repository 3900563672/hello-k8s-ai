import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ClusterBubbleField } from '@/components/features/observatory/ClusterBubbleField'
import overviewFixture from '@/lib/mocks/fixtures/overview.json'
import type { BackendNode, BackendPod, OverviewData, TenantTraffic } from '@/types/trace.types'

const base = overviewFixture.data as unknown as OverviewData

const makeNode = (name: string, overrides: Partial<BackendNode> = {}): BackendNode => ({
    ref: { apiVersion: 'v1', kind: 'Node', name },
    role: 'worker',
    ready: true,
    phase: 'Running',
    schedulable: true,
    zone: 'zone-a',
    conditions: [],
    observedAt: '2026-08-20T00:00:00.000Z',
    ...overrides,
})

const makePod = (name: string, overrides: Partial<BackendPod> = {}): BackendPod => ({
    ref: { apiVersion: 'v1', kind: 'Pod', name },
    phase: 'Running',
    ready: true,
    nodeName: 'node-a',
    tenant: 'tenant-a',
    conditions: [],
    containers: [{ name: 'sim', ready: true, restartCount: 0, state: 'running' }],
    ...overrides,
})

const makeTenant = (name: string, overrides: Partial<TenantTraffic> = {}): TenantTraffic => ({
    tenant: { apiVersion: 'hello-k8s-ai.io/v1', kind: 'Tenant', name },
    displayName: name,
    priority: 'P1',
    requestedQPS: 10,
    allocatedQPS: 10,
    allocationBalanced: true,
    performance: { sampleCount: 1, freshness: 'fresh' },
    readyReplicaCount: 1,
    instances: [],
    ...overrides,
})

const buildOverview = (): OverviewData => ({
    ...base,
    traffic: {
        ...base.traffic,
        tenants: [
            makeTenant('tenant-a'),
            makeTenant('tenant-b', { readyReplicaCount: 0, runtimePhase: 'failed' }),
        ],
    },
    workloads: {
        ...base.workloads,
        nodes: [
            makeNode('node-a'),
            makeNode('node-b', { ready: false, phase: 'Unknown' }),
        ],
        pods: [
            makePod('pod-001'),
            makePod('pod-002', { ready: false, phase: 'CrashLoopBackOff' }),
            makePod('pod-003', { ready: false, phase: 'Pending' }),
        ],
        events: [],
    },
})

describe('ClusterBubbleField', () => {
    it('无数据时渲染空计数筛选条', () => {
        render(<ClusterBubbleField overview={undefined} />)
        expect(screen.getByRole('button', { name: '节点 0' })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'Pod 0' })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: '租户 0' })).toBeInTheDocument()
    })

    it('按健康分组渲染 Pod 气泡', () => {
        render(<ClusterBubbleField overview={buildOverview()} />)
        expect(screen.getByRole('button', { name: '节点 2' })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: 'Pod 3' })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: '租户 2' })).toBeInTheDocument()
        // 默认 Pod 视图：健康分组标题（严重 1 / 关注 1 / 健康 1）
        expect(screen.getByText('严重 1')).toBeInTheDocument()
        expect(screen.getByText('关注 1')).toBeInTheDocument()
        expect(screen.getByText('健康 1')).toBeInTheDocument()
        // 气泡按钮带真实名称提示
        expect(screen.getByTitle(/真实名称：pod-001/)).toBeInTheDocument()
    })

    it('切换到节点视图并打开详情抽屉', async () => {
        const user = userEvent.setup()
        render(<ClusterBubbleField overview={buildOverview()} />)
        await user.click(screen.getByRole('button', { name: '节点 2' }))
        const healthyNode = screen.getByTitle(/node-a\s+节点 · 健康/)
        expect(healthyNode).toBeInTheDocument()
        await user.click(healthyNode)
        expect(await screen.findByText('基本信息')).toBeInTheDocument()
        expect(screen.getByText('Agent 解析')).toBeInTheDocument()
    })

    it('租户视图按运行态着色并展示详情', async () => {
        const user = userEvent.setup()
        render(<ClusterBubbleField overview={buildOverview()} />)
        await user.click(screen.getByRole('button', { name: '租户 2' }))
        const failedTenant = screen.getByTitle(/tenant-b\s+租户 · 严重/)
        expect(failedTenant).toBeInTheDocument()
        await user.click(failedTenant)
        expect(await screen.findByText('请求 QPS')).toBeInTheDocument()
    })
})
