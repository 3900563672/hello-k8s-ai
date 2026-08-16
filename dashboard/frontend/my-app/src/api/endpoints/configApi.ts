import { apiData } from '@/api/client'
import { createClientId } from '@/lib/clientId'
import type {
    BackendResource,
    ConfigurationReadModel,
    Model,
    ModelSpec,
    Node,
    NodeSpec,
    Orchestrator,
    OrchestratorSpec,
    Tenant,
    TenantSpec,
} from '@/types/config.types'

interface OperationReceipt {
    operationId: string
    acceptedAt: string
    state: string
    results: Array<{
        ref: { kind: string; name: string }
        action: string
        resourceVersion?: string
        convergence: string
    }>
}

type ConfigEntity = Model | Node | Tenant | Orchestrator

const idempotencyKey = () => createClientId('dashboard')

const configurationPath = (timestamp?: string) =>
    timestamp ? `/configuration?at=${encodeURIComponent(timestamp)}` : '/configuration'

const toModel = (resource: BackendResource<ModelSpec & Record<string, unknown>>): Model => ({
    name: resource.ref.name,
    uid: resource.ref.uid,
    resourceVersion: resource.metadata.resourceVersion,
    displayName: resource.spec.displayName,
    gpuUnits: resource.spec.gpuUnits,
    maxConcurrency: resource.spec.maxConcurrency,
    absoluteScore: resource.spec.absoluteScore
        ?? (typeof resource.status.absoluteScore === 'number' ? resource.status.absoluteScore : 0),
    coldStartMs: resource.spec.coldStartMs,
    performance: resource.spec.performance,
    status: resource.status,
    conditions: resource.conditions,
    derived: resource.derived,
})

const toNode = (resource: BackendResource<NodeSpec & Record<string, unknown>>): Node => ({
    name: resource.ref.name,
    uid: resource.ref.uid,
    resourceVersion: resource.metadata.resourceVersion,
    displayName: resource.spec.displayName,
    gpu: resource.spec.gpu,
    maxConcurrency: resource.spec.maxConcurrency,
    status: resource.status,
    conditions: resource.conditions,
    derived: resource.derived,
})

const toTenant = (resource: BackendResource<TenantSpec & Record<string, unknown>>): Tenant => ({
    name: resource.ref.name,
    uid: resource.ref.uid,
    resourceVersion: resource.metadata.resourceVersion,
    displayName: resource.spec.displayName,
    priority: resource.spec.priority,
    qps: resource.spec.qps,
    ttftThresholdMs: resource.spec.ttftThresholdMs,
    queueThreshold: resource.spec.queueThreshold,
    ttftScaleDownThresholdMs: resource.spec.ttftScaleDownThresholdMs,
    queueScaleDownThreshold: resource.spec.queueScaleDownThreshold,
    status: resource.status,
    conditions: resource.conditions,
    derived: resource.derived,
})

const modelSpec = (model: Model): ModelSpec => ({
    displayName: model.displayName,
    gpuUnits: model.gpuUnits,
    maxConcurrency: model.maxConcurrency,
    absoluteScore: model.absoluteScore,
    coldStartMs: model.coldStartMs,
    performance: model.performance,
})

const nodeSpec = (node: Node): NodeSpec => ({
    displayName: node.displayName,
    gpu: node.gpu,
    maxConcurrency: node.maxConcurrency,
})

const toOrchestrator = (resource: BackendResource<OrchestratorSpec & Record<string, unknown>>): Orchestrator => ({
    name: resource.ref.name,
    displayName: resource.spec.tenantRef?.name ?? resource.ref.name,
    tenantRef: resource.spec.tenantRef ?? { name: '' },
    scaleUpCooldownSeconds: resource.spec.scaleUpCooldownSeconds ?? 0,
    scaleDownCooldownSeconds: resource.spec.scaleDownCooldownSeconds ?? 0,
    allowScaleToZero: resource.spec.allowScaleToZero ?? false,
    minReplicas: resource.spec.minReplicas ?? 0,
    maxReplicas: resource.spec.maxReplicas ?? 0,
    status: resource.status,
    conditions: resource.conditions,
    derived: resource.derived,
})

const tenantSpec = (tenant: Tenant): TenantSpec => ({
    displayName: tenant.displayName,
    priority: tenant.priority,
    qps: tenant.qps,
    ttftThresholdMs: tenant.ttftThresholdMs,
    queueThreshold: tenant.queueThreshold,
    ttftScaleDownThresholdMs: tenant.ttftScaleDownThresholdMs,
    queueScaleDownThreshold: tenant.queueScaleDownThreshold,
})

const orchestratorSpec = (orchestrator: Orchestrator): OrchestratorSpec => ({
    tenantRef: { name: orchestrator.tenantRef.name },
    scaleUpCooldownSeconds: orchestrator.scaleUpCooldownSeconds,
    scaleDownCooldownSeconds: orchestrator.scaleDownCooldownSeconds,
    allowScaleToZero: orchestrator.allowScaleToZero,
    minReplicas: orchestrator.minReplicas,
    maxReplicas: orchestrator.maxReplicas,
})

async function applyResource<T extends ConfigEntity>(
    kind: 'Model' | 'WorkerNode' | 'Tenant' | 'Orchestrator',
    resource: T,
    spec: ModelSpec | NodeSpec | TenantSpec | OrchestratorSpec,
): Promise<T> {
    const receipt = await apiData<OperationReceipt>('/configuration:apply', {
        method: 'POST',
        headers: {
            'Idempotency-Key': idempotencyKey(),
        },
        body: JSON.stringify({
            resources: [{
                kind,
                name: resource.name,
                spec,
                resourceVersion: resource.resourceVersion,
            }],
            dryRun: false,
        }),
    })
    const result = receipt.results[0]
    return {
        ...resource,
        ...(result?.resourceVersion ? { resourceVersion: result.resourceVersion } : {}),
    }
}

async function deleteResource(kind: string, resource: ConfigEntity): Promise<void> {
    await apiData<OperationReceipt>(
        `/configuration/${encodeURIComponent(kind)}/${encodeURIComponent(resource.name)}`,
        {
            method: 'DELETE',
            headers: {
                'Idempotency-Key': idempotencyKey(),
                ...(resource.resourceVersion
                    ? { 'If-Match': `"${resource.resourceVersion}"` }
                    : {}),
            },
        },
    )
}

export const configApi = {
    getConfiguration: (timestamp?: string): Promise<ConfigurationReadModel> =>
        apiData<ConfigurationReadModel>(configurationPath(timestamp)),

    getModels: async (timestamp?: string): Promise<Model[]> =>
        (await configApi.getConfiguration(timestamp)).models.map(toModel),
    createModel: (model: Model) => applyResource('Model', model, modelSpec(model)),
    updateModel: (model: Model) => applyResource('Model', model, modelSpec(model)),
    deleteModel: async (name: string): Promise<void> => {
        const current = (await configApi.getModels()).find((item) => item.name === name)
        if (!current) return
        await deleteResource('Model', current)
    },
    deleteModels: async (names: string[]): Promise<string[]> => {
        const resources = await configApi.getModels()
        await Promise.all(resources.filter((item) => names.includes(item.name)).map((item) => deleteResource('Model', item)))
        return names
    },

    getNodes: async (timestamp?: string): Promise<Node[]> =>
        (await configApi.getConfiguration(timestamp)).workerNodes.map(toNode),
    createNode: (node: Node) => applyResource('WorkerNode', node, nodeSpec(node)),
    updateNode: (node: Node) => applyResource('WorkerNode', node, nodeSpec(node)),
    deleteNode: async (name: string): Promise<void> => {
        const current = (await configApi.getNodes()).find((item) => item.name === name)
        if (!current) return
        await deleteResource('WorkerNode', current)
    },
    deleteNodes: async (names: string[]): Promise<string[]> => {
        const resources = await configApi.getNodes()
        await Promise.all(resources.filter((item) => names.includes(item.name)).map((item) => deleteResource('WorkerNode', item)))
        return names
    },

    getTenants: async (timestamp?: string): Promise<Tenant[]> =>
        (await configApi.getConfiguration(timestamp)).tenants.map(toTenant),
    createTenant: (tenant: Tenant) => applyResource('Tenant', tenant, tenantSpec(tenant)),
    updateTenant: (tenant: Tenant) => applyResource('Tenant', tenant, tenantSpec(tenant)),
    deleteTenant: async (name: string): Promise<void> => {
        const current = (await configApi.getTenants()).find((item) => item.name === name)
        if (!current) return
        await deleteResource('Tenant', current)
    },
    deleteTenants: async (names: string[]): Promise<string[]> => {
        const resources = await configApi.getTenants()
        await Promise.all(resources.filter((item) => names.includes(item.name)).map((item) => deleteResource('Tenant', item)))
        return names
    },

    getOrchestrators: async (timestamp?: string): Promise<Orchestrator[]> =>
        (await configApi.getConfiguration(timestamp)).orchestrators.map(toOrchestrator),
    createOrchestrator: (orchestrator: Orchestrator) => applyResource('Orchestrator', orchestrator, orchestratorSpec(orchestrator)),
    updateOrchestrator: (orchestrator: Orchestrator) => applyResource('Orchestrator', orchestrator, orchestratorSpec(orchestrator)),
    deleteOrchestrator: async (name: string): Promise<void> => {
        const current = (await configApi.getOrchestrators()).find((item) => item.name === name)
        if (!current) return
        await deleteResource('Orchestrator', current)
    },
    deleteOrchestrators: async (names: string[]): Promise<string[]> => {
        const resources = await configApi.getOrchestrators()
        await Promise.all(resources.filter((item) => names.includes(item.name)).map((item) => deleteResource('Orchestrator', item)))
        return names
    },
}
