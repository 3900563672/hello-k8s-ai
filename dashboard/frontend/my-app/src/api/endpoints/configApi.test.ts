// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { configApi } from '@/api/endpoints/configApi'
import type { Model, Policy, Tenant } from '@/types/config.types'

const okResponse = (body: unknown, status = 200): Response =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const resource = (kind: string, name: string, spec: Record<string, unknown>, extra: Record<string, unknown> = {}) => ({
    ref: { apiVersion: 'v1', kind, name, uid: `uid-${name}` },
    metadata: { generation: 1, resourceVersion: '7' },
    spec,
    status: { phase: 'Ready' },
    conditions: [],
    derived: {},
    ...extra,
})

const configuration = () => ({
    data: {
        asOf: '2026-08-20T00:00:00.000Z',
        availability: 'available',
        models: [
            resource('Model', 'model-a', {
                displayName: '模型A', gpuUnits: 2, maxConcurrency: 4,
                absoluteScore: 80, coldStartMs: 100,
                performance: { prefillBaseMs: 10, prefillPerTokenUs: 5, decodePerTokenMs: 3 },
            }),
            resource('Model', 'model-legacy', {
                displayName: '旧模型', gpuUnits: 1, maxConcurrency: 2,
                coldStartMs: 50,
                performance: { prefillBaseMs: 20, prefillPerTokenUs: 8, decodePerTokenMs: 5 },
            }, { status: { absoluteScore: 90 } }),
        ],
        workerNodes: [
            resource('WorkerNode', 'node-a', { displayName: '节点A', gpu: 8, maxConcurrency: 16 }),
        ],
        tenants: [
            resource('Tenant', 'tenant-a', {
                displayName: '租户A', priority: 'P1', qps: 100,
                ttftThresholdMs: 2000, queueThreshold: 50,
                ttftScaleDownThresholdMs: 1000, queueScaleDownThreshold: 20,
            }),
        ],
        orchestrators: [
            resource('Orchestrator', 'orch-a', {
                tenantRef: { name: 'tenant-a' }, scaleUpCooldownSeconds: 30,
                scaleDownCooldownSeconds: 30, allowScaleToZero: true,
                minReplicas: 1, maxReplicas: 3, maxScaleUpBatch: 1,
            }),
        ],
        policies: {
            tenantModel: [resource('TenantModelPolicy', 'p-tm', { tenantRef: { name: 'tenant-a' }, modelRef: { name: 'model-a' }, effect: 'Allow' })],
            tenantNode: [resource('TenantNodePolicy', 'p-tn', { tenantRef: { name: 'tenant-a' }, nodeRef: { name: 'node-a' }, effect: 'Deny' })],
            modelNode: [resource('ModelNodePolicy', 'p-mn', { modelRef: { name: 'model-a' }, nodeRef: { name: 'node-a' }, effect: 'Allow' })],
        },
        simulationClocks: [],
    },
    meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
})

describe('configApi', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => { originalFetch = globalThis.fetch })
    afterEach(() => { globalThis.fetch = originalFetch })

    const mockCalls = () => (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls

    it('getConfiguration 无时间戳走裸路径，有时间戳带 at', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(configuration()))
        await configApi.getConfiguration()
        expect(String(mockCalls()[0][0])).toBe('/api/v1/configuration')
        await configApi.getConfiguration('2026-08-20T00:00:00.000Z')
        expect(String(mockCalls()[1][0])).toContain('/configuration?at=2026-08-20T00%3A00%3A00.000Z')
    })

    it('getModels 映射字段，absoluteScore 缺失时回退 status', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(configuration()))
        const models = await configApi.getModels()
        expect(models).toHaveLength(2)
        expect(models[0]).toMatchObject({ name: 'model-a', displayName: '模型A', gpuUnits: 2, absoluteScore: 80, uid: 'uid-model-a', resourceVersion: '7' })
        expect(models[1].absoluteScore).toBe(90)
    })

    it('createModel POST :apply 组装 body 并合并回执 resourceVersion', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({
            data: { operationId: 'op-1', acceptedAt: '2026-08-20T00:00:00.000Z', state: 'applied', results: [{ ref: { kind: 'Model', name: 'model-a' }, action: 'create', resourceVersion: '9', convergence: 'converged' }] },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }, 202))
        const model: Model = {
            name: 'model-a', displayName: '模型A', gpuUnits: 2, maxConcurrency: 4,
            absoluteScore: 80, coldStartMs: 100,
            performance: { prefillBaseMs: 10, prefillPerTokenUs: 5, decodePerTokenMs: 3 },
            resourceVersion: '7',
        }
        const created = await configApi.createModel(model)
        expect(created.resourceVersion).toBe('9')
        const [url, init] = mockCalls()[0]
        expect(String(url)).toBe('/api/v1/configuration:apply')
        expect(init?.method).toBe('POST')
        const body = JSON.parse(String(init?.body))
        expect(body.resources[0]).toEqual({
            kind: 'Model', name: 'model-a', resourceVersion: '7',
            spec: { displayName: '模型A', gpuUnits: 2, maxConcurrency: 4, absoluteScore: 80, coldStartMs: 100, performance: { prefillBaseMs: 10, prefillPerTokenUs: 5, decodePerTokenMs: 3 } },
        })
        expect(body.dryRun).toBe(false)
        expect(new Headers(init?.headers).get('Idempotency-Key')).toMatch(/^dashboard-/)
    })

    it('deleteModel 存在时 DELETE 带 If-Match；不存在时静默', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(configuration()))
        await configApi.deleteModel('model-a')
        const [url, init] = mockCalls()[1]
        expect(String(url)).toBe('/api/v1/configuration/Model/model-a')
        expect(init?.method).toBe('DELETE')
        expect(new Headers(init?.headers).get('If-Match')).toBe('"7"')

        await configApi.deleteModel('model-missing')
        expect(mockCalls()).toHaveLength(3)
    })

    it('deleteModels 只删除命中的资源并返回入参', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(configuration()))
        const result = await configApi.deleteModels(['model-a', 'model-missing'])
        expect(result).toEqual(['model-a', 'model-missing'])
        expect(mockCalls()).toHaveLength(2) // getConfiguration + 1 DELETE
    })

    it('getNodes/getTenants/getOrchestrators 映射后端资源', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(configuration()))
        const nodes = await configApi.getNodes()
        expect(nodes[0]).toMatchObject({ name: 'node-a', displayName: '节点A', gpu: 8, maxConcurrency: 16 })
        const tenants = await configApi.getTenants()
        expect(tenants[0]).toMatchObject({ name: 'tenant-a', displayName: '租户A', priority: 'P1', qps: 100 })
        const orchestrators = await configApi.getOrchestrators()
        expect(orchestrators[0]).toMatchObject({ name: 'orch-a', displayName: 'tenant-a', tenantRef: { name: 'tenant-a' }, allowScaleToZero: true, minReplicas: 1, maxReplicas: 3 })
    })

    it('createOrchestrator 无 tenantRef 时用 ref.name 兜底', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(configuration()))
        const orchestrators = await configApi.getOrchestrators()
        expect(orchestrators[0].displayName).toBe('tenant-a')
    })

    it('getPolicies 展平三类策略并生成展示名', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(configuration()))
        const policies = await configApi.getPolicies()
        expect(policies).toHaveLength(3)
        expect(policies[0]).toMatchObject({ name: 'p-tm', kind: 'tenantModel', displayName: 'tenant-a → model-a', effect: 'Allow' })
        expect(policies[1]).toMatchObject({ kind: 'tenantNode', displayName: 'tenant-a → node-a', effect: 'Deny' })
        expect(policies[2]).toMatchObject({ kind: 'modelNode', displayName: 'model-a → node-a' })
    })

    it('createPolicy/updatePolicy 映射 CR kind，deletePolicy 直接删除', async () => {
        const policy: Policy = {
            name: 'p-tm', displayName: 'tenant-a → model-a', kind: 'tenantModel',
            tenantRef: { name: 'tenant-a' }, modelRef: { name: 'model-a' },
            effect: 'Allow', resourceVersion: '7',
        }
        globalThis.fetch = vi.fn(async () => okResponse({
            data: { operationId: 'op-1', acceptedAt: '2026-08-20T00:00:00.000Z', state: 'applied', results: [{ ref: { kind: 'TenantModelPolicy', name: 'p-tm' }, action: 'create', convergence: 'converged' }] },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }, 202))
        await configApi.createPolicy(policy)
        const body = JSON.parse(String(mockCalls()[0][1]?.body))
        expect(body.resources[0].kind).toBe('TenantModelPolicy')
        expect(body.resources[0].spec).toEqual({ tenantRef: { name: 'tenant-a' }, modelRef: { name: 'model-a' }, effect: 'Allow' })

        await configApi.updatePolicy(policy)
        const body2 = JSON.parse(String(mockCalls()[1][1]?.body))
        expect(body2.resources[0].kind).toBe('TenantModelPolicy')

        await configApi.deletePolicy(policy)
        expect(String(mockCalls()[2][0])).toBe('/api/v1/configuration/TenantModelPolicy/p-tm')
        expect(mockCalls()[2][1]?.method).toBe('DELETE')
    })

    it('createTenant/updateTenant 组装 tenant spec', async () => {
        const tenant: Tenant = {
            name: 'tenant-a', displayName: '租户A', priority: 'P1', qps: 100,
            ttftThresholdMs: 2000, queueThreshold: 50,
            ttftScaleDownThresholdMs: 1000, queueScaleDownThreshold: 20,
        }
        globalThis.fetch = vi.fn(async () => okResponse({
            data: { operationId: 'op-1', acceptedAt: '2026-08-20T00:00:00.000Z', state: 'applied', results: [{ ref: { kind: 'Tenant', name: 'tenant-a' }, action: 'create', convergence: 'converged' }] },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }, 202))
        await configApi.updateTenant(tenant)
        const body = JSON.parse(String(mockCalls()[0][1]?.body))
        expect(body.resources[0]).toEqual({
            kind: 'Tenant', name: 'tenant-a', resourceVersion: undefined,
            spec: {
                displayName: '租户A', priority: 'P1', qps: 100,
                ttftThresholdMs: 2000, queueThreshold: 50,
                ttftScaleDownThresholdMs: 1000, queueScaleDownThreshold: 20,
            },
        })
    })
})