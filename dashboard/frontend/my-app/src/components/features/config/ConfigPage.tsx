import { useEffect, useMemo, useState } from 'react'
import { BrainCircuit, DatabaseZap, History, RefreshCw, RotateCcw, Server, Settings2, ShieldCheck, SlidersHorizontal, Users } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { ConfigTabPanel } from './components/ConfigTabPanel'
import { ModelTable } from './tables/ModelTable'
import { NodeTable } from './tables/NodeTable'
import { OrchestratorTable } from './tables/OrchestratorTable'
import { PolicyTable } from './tables/PolicyTable'
import { TenantTable } from './tables/TenantTable'
import { ModelForm } from './forms/ModelForm'
import { NodeForm } from './forms/NodeForm'
import { OrchestratorForm } from './forms/OrchestratorForm'
import { PolicyForm } from './forms/PolicyForm'
import { TenantForm } from './forms/TenantForm'
import { CreateDialog } from '@/components/shared/dialogs/CreateDialog'
import { RenameDialog } from '@/components/shared/dialogs/RenameDialog'
import { BatchDeleteDialog } from '@/components/shared/dialogs/BatchDeleteDialog'
import { PolicyCreateDialog } from '@/components/shared/dialogs/PolicyCreateDialog'
import { TemplateLibraryDialog } from '@/components/shared/dialogs/TemplateLibraryDialog'
import {
    useCreateModel,
    useCreateNode,
    useCreateOrchestrator,
    useCreatePolicy,
    useCreateTenant,
    useDeleteModel,
    useDeleteModels,
    useDeleteNode,
    useDeleteNodes,
    useDeleteOrchestrator,
    useDeleteOrchestrators,
    useDeletePolicy,
    useDeleteTenant,
    useDeleteTenants,
    useModels,
    useNodes,
    useOrchestrators,
    usePolicies,
    useTenants,
    useUpdateModel,
    useUpdateNode,
    useUpdateOrchestrator,
    useUpdatePolicy,
    useUpdateTenant,
} from '@/api/queries/configQueries'
import type { PolicyFormValues } from '@/lib/validations/policy.schema'
import type { ModelFormValues } from '@/lib/validations/model.schema'
import { getModelPreview } from '@/lib/validations/model.schema'
import type { NodeFormValues } from '@/lib/validations/node.schema'
import { getNodePreview } from '@/lib/validations/node.schema'
import type { OrchestratorFormValues } from '@/lib/validations/orchestrator.schema'
import { getOrchestratorPreview } from '@/lib/validations/orchestrator.schema'
import type { TenantFormValues } from '@/lib/validations/tenant.schema'
import { getTenantPreview } from '@/lib/validations/tenant.schema'
import type {
    ConfigResourceType,
    Model,
    Node,
    Orchestrator,
    Policy,
    PolicyEffect,
    PolicyKind,
    Tenant,
} from '@/types/config.types'
import { useWorkspaceContext } from '@/hooks/useWorkspaceContext'
import { useTemplateStore } from '@/stores/templateSlice'
import type { ConnectionStatus } from '@/types/control-plane.types'
import { useTimeStore } from '@/stores/timeSlice'
import { formatUtcTimestamp } from '@/lib/formatters/timeFormatter'
import { DEFAULT_MODEL, DEFAULT_NODE, DEFAULT_ORCHESTRATOR, DEFAULT_TENANT } from '@/lib/constants/defaultValues'

type ConfigTab = 'models' | 'nodes' | 'tenants' | 'orchestrators' | 'policies'
type RenameTarget =
    | { type: 'model'; resource: Model }
    | { type: 'node'; resource: Node }
    | { type: 'tenant'; resource: Tenant }

type CreateTemplateSource =
    | { type: 'model'; data: ModelFormValues }
    | { type: 'node'; data: NodeFormValues }
    | { type: 'tenant'; data: TenantFormValues }
    | { type: 'orchestrator'; data: OrchestratorFormValues }

// 连接状态来自 controlPlaneStore 的 cluster 快照，与 ClusterStatus 展示口径一致
const connectionMeta: Record<ConnectionStatus, { label: string; dot: string; badge: string }> = {
    connected: {
        label: 'Backend 已连接',
        dot: 'bg-[#57C894] shadow-[0_0_8px_rgba(87,200,148,0.8)]',
        badge: 'border-emerald-500/20 bg-emerald-500/5 text-[#72CFA2]',
    },
    connecting: {
        label: 'Backend 连接中',
        dot: 'bg-amber-400',
        badge: 'border-amber-400/20 bg-amber-400/5 text-amber-300',
    },
    disconnected: {
        label: 'Backend 未连接',
        dot: 'bg-red-400',
        badge: 'border-red-400/20 bg-red-400/5 text-red-300',
    },
}

const resourceLabels: Record<ConfigResourceType, string> = {
    model: '模型',
    node: '节点',
    tenant: '租户',
    orchestrator: '编排策略',
}

// 首次请求完成前 TanStack Query 会返回 undefined。使用稳定空数组，避免选择逻辑在每次
// 渲染时观察到新数组并形成渲染循环。
const EMPTY_MODELS: Model[] = []
const EMPTY_NODES: Node[] = []
const EMPTY_TENANTS: Tenant[] = []
const EMPTY_ORCHESTRATORS: Orchestrator[] = []
const EMPTY_POLICIES: Policy[] = []

const mutationError = (error: unknown): string =>
    error instanceof Error ? error.message : '操作失败，请稍后重试'

const useResourceSelection = <T extends { name: string }>(items: T[]) => {
    const [selectedName, setSelectedName] = useState<string | null>(null)

    useEffect(() => {
        setSelectedName((current) => {
            if (current && items.some((item) => item.name === current)) return current
            return items[0]?.name ?? null
        })
    }, [items])

    const selectedItem = useMemo(
        () => items.find((item) => item.name === selectedName) ?? null,
        [items, selectedName],
    )

    return { selectedItem, setSelectedName }
}

const normalizeIdentifier = (value: string): string =>
    value
        .trim()
        .normalize('NFKC')
        .toLocaleLowerCase()
        .replace(/[^\p{L}\p{N}]+/gu, '-')
        .replace(/^-+|-+$/g, '')

const uniqueIdentifier = (value: string, type: ConfigResourceType, existingIds: string[]): string => {
    const prefix = type === 'model' ? 'model' : type === 'node' ? 'node' : type === 'tenant' ? 'tenant' : 'orch'
    const normalized = normalizeIdentifier(value)
    const base = normalized || `${prefix}-${Date.now().toString(36)}`
    const existing = new Set(existingIds)
    if (!existing.has(base)) return base

    let suffix = 2
    while (existing.has(`${base}-${suffix}`)) suffix += 1
    return `${base}-${suffix}`
}

const modelFormValues = (model: Model): ModelFormValues => ({
    displayName: model.displayName,
    gpuUnits: model.gpuUnits,
    maxConcurrency: model.maxConcurrency,
    absoluteScore: model.absoluteScore,
    coldStartMs: model.coldStartMs,
    performance: { ...model.performance },
})

const nodeFormValues = (node: Node): NodeFormValues => ({
    displayName: node.displayName,
    gpu: node.gpu,
    maxConcurrency: node.maxConcurrency,
})

const tenantFormValues = (tenant: Tenant): TenantFormValues => ({
    displayName: tenant.displayName,
    priority: tenant.priority,
    qps: tenant.qps,
    ttftThresholdMs: tenant.ttftThresholdMs,
    queueThreshold: tenant.queueThreshold,
    ttftScaleDownThresholdMs: tenant.ttftScaleDownThresholdMs,
    queueScaleDownThreshold: tenant.queueScaleDownThreshold,
})

const orchestratorFormValues = (orchestrator: Orchestrator): OrchestratorFormValues => ({
    tenantName: orchestrator.tenantRef.name,
    scaleUpCooldownSeconds: orchestrator.scaleUpCooldownSeconds,
    scaleDownCooldownSeconds: orchestrator.scaleDownCooldownSeconds,
    allowScaleToZero: orchestrator.allowScaleToZero,
    minReplicas: orchestrator.minReplicas,
    maxReplicas: orchestrator.maxReplicas,
})

const policyFormValues = (policy: Policy): PolicyFormValues => ({
    kind: policy.kind,
    tenantName: policy.tenantRef?.name ?? '',
    modelName: policy.modelRef?.name ?? '',
    nodeName: policy.nodeRef?.name ?? '',
    effect: policy.effect,
})

const policyDisplayName = (kind: PolicyKind, tenantName: string, modelName: string, nodeName: string): string => {
    if (kind === 'tenantModel') return `${tenantName} → ${modelName}`
    if (kind === 'tenantNode') return `${tenantName} → ${nodeName}`
    return `${modelName} → ${nodeName}`
}

export function ConfigPage() {
    const [activeTab, setActiveTab] = useState<ConfigTab>('models')
    const workspace = useWorkspaceContext()
    const returnToLatest = useTimeStore((state) => state.returnToLatest)
    const readOnly = workspace.isHistorical
    const connectionStatus = workspace.cluster.connectionStatus

    const modelsQuery = useModels()
    const nodesQuery = useNodes()
    const tenantsQuery = useTenants()
    const orchestratorsQuery = useOrchestrators()
    const policiesQuery = usePolicies()
    const models = modelsQuery.data ?? EMPTY_MODELS
    const nodes = nodesQuery.data ?? EMPTY_NODES
    const tenants = tenantsQuery.data ?? EMPTY_TENANTS
    const orchestrators = orchestratorsQuery.data ?? EMPTY_ORCHESTRATORS
    const policies = policiesQuery.data ?? EMPTY_POLICIES

    const {
        modelTemplates,
        nodeTemplates,
        tenantTemplates,
        orchestratorTemplates,
        removeModelTemplate,
        removeNodeTemplate,
        removeTenantTemplate,
        removeOrchestratorTemplate,
    } = useTemplateStore()

    const createModel = useCreateModel()
    const updateModel = useUpdateModel()
    const deleteModel = useDeleteModel()
    const deleteModels = useDeleteModels()
    const createNode = useCreateNode()
    const updateNode = useUpdateNode()
    const deleteNode = useDeleteNode()
    const deleteNodes = useDeleteNodes()
    const createTenant = useCreateTenant()
    const updateTenant = useUpdateTenant()
    const deleteTenant = useDeleteTenant()
    const deleteTenants = useDeleteTenants()
    const createOrchestrator = useCreateOrchestrator()
    const updateOrchestrator = useUpdateOrchestrator()
    const deleteOrchestrator = useDeleteOrchestrator()
    const deleteOrchestrators = useDeleteOrchestrators()
    const createPolicy = useCreatePolicy()
    const updatePolicy = useUpdatePolicy()
    const deletePolicy = useDeletePolicy()

    const modelSelection = useResourceSelection(models)
    const nodeSelection = useResourceSelection(nodes)
    const tenantSelection = useResourceSelection(tenants)
    const orchestratorSelection = useResourceSelection(orchestrators)
    const policySelection = useResourceSelection(policies)

    const [selectedModels, setSelectedModels] = useState<string[]>([])
    const [selectedNodes, setSelectedNodes] = useState<string[]>([])
    const [selectedTenants, setSelectedTenants] = useState<string[]>([])
    const [selectedOrchestrators, setSelectedOrchestrators] = useState<string[]>([])
    const [selectedPolicies, setSelectedPolicies] = useState<string[]>([])

    useEffect(() => {
        const ids = new Set(models.map((model) => model.name))
        setSelectedModels((selected) => {
            const next = selected.filter((id) => ids.has(id))
            return next.length === selected.length ? selected : next
        })
    }, [models])
    useEffect(() => {
        const ids = new Set(nodes.map((node) => node.name))
        setSelectedNodes((selected) => {
            const next = selected.filter((id) => ids.has(id))
            return next.length === selected.length ? selected : next
        })
    }, [nodes])
    useEffect(() => {
        const ids = new Set(tenants.map((tenant) => tenant.name))
        setSelectedTenants((selected) => {
            const next = selected.filter((id) => ids.has(id))
            return next.length === selected.length ? selected : next
        })
    }, [tenants])
    useEffect(() => {
        const ids = new Set(orchestrators.map((orchestrator) => orchestrator.name))
        setSelectedOrchestrators((selected) => {
            const next = selected.filter((id) => ids.has(id))
            return next.length === selected.length ? selected : next
        })
    }, [orchestrators])
    useEffect(() => {
        const ids = new Set(policies.map((policy) => policy.name))
        setSelectedPolicies((selected) => {
            const next = selected.filter((id) => ids.has(id))
            return next.length === selected.length ? selected : next
        })
    }, [policies])

    const [createOpen, setCreateOpen] = useState(false)
    const [createType, setCreateType] = useState<ConfigResourceType>('model')
    const [newName, setNewName] = useState('')
    const [createError, setCreateError] = useState('')
    const [createFromTemplateOpen, setCreateFromTemplateOpen] = useState(false)
    const [createTemplateSource, setCreateTemplateSource] = useState<CreateTemplateSource | null>(null)

    const [renameTarget, setRenameTarget] = useState<RenameTarget | null>(null)
    const [renameValue, setRenameValue] = useState('')
    const [renameError, setRenameError] = useState('')

    const [batchDeleteType, setBatchDeleteType] = useState<ConfigResourceType | null>(null)
    const [batchDeleteError, setBatchDeleteError] = useState('')

    const [policyCreateOpen, setPolicyCreateOpen] = useState(false)
    const [policyCreateKind, setPolicyCreateKind] = useState<PolicyKind>('tenantModel')
    const [policyTenantName, setPolicyTenantName] = useState('')
    const [policyModelName, setPolicyModelName] = useState('')
    const [policyNodeName, setPolicyNodeName] = useState('')
    const [policyEffect, setPolicyEffect] = useState<PolicyEffect>('Allow')
    const [policyCreateError, setPolicyCreateError] = useState('')
    const [policyBatchDeleteOpen, setPolicyBatchDeleteOpen] = useState(false)
    const [policyBatchDeleteError, setPolicyBatchDeleteError] = useState('')

    useEffect(() => {
        if (!readOnly) return
        setSelectedModels([])
        setSelectedNodes([])
        setSelectedTenants([])
        setSelectedOrchestrators([])
        setSelectedPolicies([])
        setCreateOpen(false)
        setCreateFromTemplateOpen(false)
        setCreateTemplateSource(null)
        setRenameTarget(null)
        setBatchDeleteType(null)
        setPolicyCreateOpen(false)
        setPolicyBatchDeleteOpen(false)
    }, [readOnly])

    const existingIds =
        createType === 'model'
            ? models.map((item) => item.name)
            : createType === 'node'
              ? nodes.map((item) => item.name)
              : createType === 'tenant'
                ? tenants.map((item) => item.name)
                : orchestrators.map((item) => item.name)
    const identifierPreview = uniqueIdentifier(newName, createType, existingIds)

    const createPending =
        createType === 'model'
            ? createModel.isPending
            : createType === 'node'
              ? createNode.isPending
              : createType === 'tenant'
                ? createTenant.isPending
                : createOrchestrator.isPending

    const renamePending =
        renameTarget?.type === 'model'
            ? updateModel.isPending
            : renameTarget?.type === 'node'
              ? updateNode.isPending
              : renameTarget?.type === 'tenant'
                ? updateTenant.isPending
                : false

    const batchDeletePending =
        batchDeleteType === 'model'
            ? deleteModels.isPending
            : batchDeleteType === 'node'
              ? deleteNodes.isPending
              : batchDeleteType === 'tenant'
                ? deleteTenants.isPending
                : batchDeleteType === 'orchestrator'
                  ? deleteOrchestrators.isPending
                  : false

    const openCreate = (type: ConfigResourceType) => {
        if (readOnly) return
        setCreateType(type)
        setNewName('')
        setCreateError('')
        setCreateTemplateSource(null)
        setCreateOpen(true)
    }

    const openCreateFromTemplate = (type: ConfigResourceType) => {
        if (readOnly) return
        setCreateType(type)
        setCreateError('')
        setCreateFromTemplateOpen(true)
    }

    const applyCreateTemplate = (source: CreateTemplateSource) => {
        setCreateFromTemplateOpen(false)
        setCreateTemplateSource(source)
        // 预填创建弹窗名称：编排策略预填关联租户，其余预填显示名称
        setNewName(source.type === 'orchestrator' ? source.data.tenantName : source.data.displayName)
        setCreateError('')
        setCreateOpen(true)
    }

    const confirmCreate = async () => {
        if (readOnly) return
        const displayName = newName.trim()
        if (!displayName || createPending) return
        const name = uniqueIdentifier(displayName, createType, existingIds)
        setCreateError('')

        try {
            // 从模板新建时用模板数据替代 DEFAULT_* 组装；无模板时保持原有默认值行为
            const template = createTemplateSource
            if (createType === 'model') {
                const model: Model = template?.type === 'model'
                    ? {
                        name,
                        displayName,
                        gpuUnits: template.data.gpuUnits,
                        maxConcurrency: template.data.maxConcurrency,
                        absoluteScore: template.data.absoluteScore,
                        coldStartMs: template.data.coldStartMs,
                        performance: { ...template.data.performance },
                    }
                    : {
                        name,
                        displayName,
                        ...DEFAULT_MODEL,
                        performance: { ...DEFAULT_MODEL.performance },
                    }
                await createModel.mutateAsync(model)
                modelSelection.setSelectedName(model.name)
            } else if (createType === 'node') {
                const node: Node = template?.type === 'node'
                    ? { name, ...template.data, displayName }
                    : { name, displayName, ...DEFAULT_NODE }
                await createNode.mutateAsync(node)
                nodeSelection.setSelectedName(node.name)
            } else if (createType === 'tenant') {
                const tenant: Tenant = template?.type === 'tenant'
                    ? { name, ...template.data, displayName }
                    : {
                        name,
                        displayName,
                        ...DEFAULT_TENANT,
                    }
                await createTenant.mutateAsync(tenant)
                tenantSelection.setSelectedName(tenant.name)
            } else {
                const tenantName = normalizeIdentifier(newName)
                if (!tenants.some((item) => item.name === tenantName)) {
                    setCreateError(`未找到租户“${newName.trim()}”，请先创建该租户`)
                    return
                }
                const orchestrator: Orchestrator = template?.type === 'orchestrator'
                    ? {
                        name,
                        displayName: tenantName,
                        tenantRef: { name: tenantName },
                        scaleUpCooldownSeconds: template.data.scaleUpCooldownSeconds,
                        scaleDownCooldownSeconds: template.data.scaleDownCooldownSeconds,
                        allowScaleToZero: template.data.allowScaleToZero,
                        minReplicas: template.data.minReplicas,
                        maxReplicas: template.data.maxReplicas,
                    }
                    : {
                        name,
                        displayName: tenantName,
                        ...DEFAULT_ORCHESTRATOR,
                        tenantRef: { name: tenantName },
                    }
                await createOrchestrator.mutateAsync(orchestrator)
                orchestratorSelection.setSelectedName(orchestrator.name)
            }
            setCreateOpen(false)
            setNewName('')
            setCreateTemplateSource(null)
        } catch (error) {
            setCreateError(mutationError(error))
        }
    }

    const openRename = (target: RenameTarget) => {
        if (readOnly) return
        setRenameTarget(target)
        setRenameValue(target.resource.displayName)
        setRenameError('')
    }

    const confirmRename = async () => {
        if (readOnly) return
        const displayName = renameValue.trim()
        if (!displayName || !renameTarget || renamePending) return
        setRenameError('')
        try {
            if (renameTarget.type === 'model') {
                await updateModel.mutateAsync({ ...renameTarget.resource, displayName })
            } else if (renameTarget.type === 'node') {
                await updateNode.mutateAsync({ ...renameTarget.resource, displayName })
            } else {
                await updateTenant.mutateAsync({ ...renameTarget.resource, displayName })
            }
            setRenameTarget(null)
        } catch (error) {
            setRenameError(mutationError(error))
        }
    }

    const openBatchDelete = (type: ConfigResourceType) => {
        if (readOnly) return
        const count =
            type === 'model'
                ? selectedModels.length
                : type === 'node'
                  ? selectedNodes.length
                  : type === 'tenant'
                    ? selectedTenants.length
                    : selectedOrchestrators.length
        if (count === 0) return
        setBatchDeleteType(type)
        setBatchDeleteError('')
    }

    const confirmBatchDelete = async () => {
        if (readOnly) return
        if (!batchDeleteType || batchDeletePending) return
        setBatchDeleteError('')
        try {
            if (batchDeleteType === 'model') {
                await deleteModels.mutateAsync(selectedModels)
                setSelectedModels([])
            } else if (batchDeleteType === 'node') {
                await deleteNodes.mutateAsync(selectedNodes)
                setSelectedNodes([])
            } else if (batchDeleteType === 'tenant') {
                await deleteTenants.mutateAsync(selectedTenants)
                setSelectedTenants([])
            } else {
                await deleteOrchestrators.mutateAsync(selectedOrchestrators)
                setSelectedOrchestrators([])
            }
            setBatchDeleteType(null)
        } catch (error) {
            setBatchDeleteError(mutationError(error))
        }
    }

    const policyIdentifierPreview = (() => {
        if (policyCreateKind === 'tenantModel') {
            if (!policyTenantName || !policyModelName) return ''
            const base = `${policyTenantName}-${policyModelName}`
            const existing = new Set(policies.map((policy) => policy.name))
            if (!existing.has(base)) return base
            let suffix = 2
            while (existing.has(`${base}-${suffix}`)) suffix += 1
            return `${base}-${suffix}`
        }
        if (policyCreateKind === 'tenantNode') {
            if (!policyTenantName || !policyNodeName) return ''
            const base = `${policyTenantName}-${policyNodeName}`
            const existing = new Set(policies.map((policy) => policy.name))
            if (!existing.has(base)) return base
            let suffix = 2
            while (existing.has(`${base}-${suffix}`)) suffix += 1
            return `${base}-${suffix}`
        }
        if (!policyModelName || !policyNodeName) return ''
        const base = `${policyModelName}-${policyNodeName}`
        const existing = new Set(policies.map((policy) => policy.name))
        if (!existing.has(base)) return base
        let suffix = 2
        while (existing.has(`${base}-${suffix}`)) suffix += 1
        return `${base}-${suffix}`
    })()

    const openCreatePolicy = (kind: PolicyKind) => {
        if (readOnly) return
        setPolicyCreateKind(kind)
        setPolicyTenantName('')
        setPolicyModelName('')
        setPolicyNodeName('')
        setPolicyEffect('Allow')
        setPolicyCreateError('')
        setPolicyCreateOpen(true)
    }

    const confirmCreatePolicy = async () => {
        if (readOnly || createPolicy.isPending) return
        const name = policyIdentifierPreview
        if (!name) {
            setPolicyCreateError('请先选择策略的引用对象')
            return
        }
        setPolicyCreateError('')
        const tenantName = policyCreateKind === 'modelNode' ? '' : policyTenantName
        const modelName = policyCreateKind === 'tenantNode' ? '' : policyModelName
        const nodeName = policyCreateKind === 'tenantModel' ? '' : policyNodeName
        const policy: Policy = {
            name,
            displayName: policyDisplayName(policyCreateKind, tenantName, modelName, nodeName),
            kind: policyCreateKind,
            ...(tenantName ? { tenantRef: { name: tenantName } } : {}),
            ...(modelName ? { modelRef: { name: modelName } } : {}),
            ...(nodeName ? { nodeRef: { name: nodeName } } : {}),
            effect: policyEffect,
        }
        try {
            await createPolicy.mutateAsync(policy)
            policySelection.setSelectedName(policy.name)
            setPolicyCreateOpen(false)
        } catch (error) {
            setPolicyCreateError(mutationError(error))
        }
    }

    const savePolicy = async (values: PolicyFormValues) => {
        if (readOnly) return
        if (!policySelection.selectedItem) return
        await updatePolicy.mutateAsync({
            ...policySelection.selectedItem,
            ...(values.tenantName ? { tenantRef: { name: values.tenantName } } : {}),
            ...(values.modelName ? { modelRef: { name: values.modelName } } : {}),
            ...(values.nodeName ? { nodeRef: { name: values.nodeName } } : {}),
            effect: values.effect,
        })
    }

    const deletePolicyByName = async (name: string): Promise<void> => {
        const policy = policies.find((item) => item.name === name)
        if (!policy) return
        await deletePolicy.mutateAsync(policy)
    }

    const openBatchDeletePolicy = () => {
        if (readOnly) return
        if (selectedPolicies.length === 0) return
        setPolicyBatchDeleteError('')
        setPolicyBatchDeleteOpen(true)
    }

    const confirmPolicyBatchDelete = async () => {
        if (readOnly) return
        setPolicyBatchDeleteError('')
        try {
            await Promise.all(selectedPolicies.map((name) => deletePolicyByName(name)))
            setSelectedPolicies([])
            setPolicyBatchDeleteOpen(false)
        } catch (error) {
            setPolicyBatchDeleteError(mutationError(error))
        }
    }

    const saveModel = async (values: ModelFormValues) => {
        if (readOnly) return
        if (!modelSelection.selectedItem) return
        await updateModel.mutateAsync({ ...modelSelection.selectedItem, ...values })
    }

    const saveNode = async (values: NodeFormValues) => {
        if (readOnly) return
        if (!nodeSelection.selectedItem) return
        await updateNode.mutateAsync({ ...nodeSelection.selectedItem, ...values })
    }

    const saveTenant = async (values: TenantFormValues) => {
        if (readOnly) return
        if (!tenantSelection.selectedItem) return
        await updateTenant.mutateAsync({ ...tenantSelection.selectedItem, ...values })
    }

    const saveOrchestrator = async (values: OrchestratorFormValues) => {
        if (readOnly) return
        if (!orchestratorSelection.selectedItem) return
        await updateOrchestrator.mutateAsync({
            ...orchestratorSelection.selectedItem,
            tenantRef: { name: values.tenantName },
            scaleUpCooldownSeconds: values.scaleUpCooldownSeconds,
            scaleDownCooldownSeconds: values.scaleDownCooldownSeconds,
            allowScaleToZero: values.allowScaleToZero,
            minReplicas: values.minReplicas,
            maxReplicas: values.maxReplicas,
        })
    }

    const loading = modelsQuery.isLoading || nodesQuery.isLoading || tenantsQuery.isLoading || orchestratorsQuery.isLoading || policiesQuery.isLoading
    const queryError = modelsQuery.error ?? nodesQuery.error ?? tenantsQuery.error ?? orchestratorsQuery.error ?? policiesQuery.error
    const totalResources = models.length + nodes.length + tenants.length + orchestrators.length + policies.length
    const batchDeleteCount =
        batchDeleteType === 'model'
            ? selectedModels.length
            : batchDeleteType === 'node'
              ? selectedNodes.length
              : batchDeleteType === 'tenant'
                ? selectedTenants.length
                : batchDeleteType === 'orchestrator'
                  ? selectedOrchestrators.length
                  : 0

    if (loading) {
        return (
            <div className="flex h-full min-h-0 flex-col bg-[#05070A] p-5">
                <div className="h-20 animate-pulse rounded-xl border border-[#1B2634] bg-[#0B0B0B]" />
                <div className="mt-4 grid min-h-0 flex-1 grid-cols-1 gap-4 xl:grid-cols-[44%_1fr]">
                    <div className="animate-pulse rounded-xl border border-[#1B2634] bg-[#0B0B0B]" />
                    <div className="animate-pulse rounded-xl border border-[#1B2634] bg-[#0B0B0B]" />
                </div>
            </div>
        )
    }

    if (queryError) {
        return (
            <div className="flex h-full items-center justify-center bg-[#05070A] p-6">
                <div className="w-full max-w-md rounded-xl border border-[#272727] bg-[#0B1018] p-6 text-center">
                    <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-xl border border-red-500/20 bg-red-500/10">
                        <DatabaseZap className="h-5 w-5 text-[#FF7373]" />
                    </div>
                    <h1 className="mt-4 text-base font-semibold text-[#F0F0F0]">无法读取本地配置</h1>
                    <p className="mt-2 text-sm leading-6 text-[#748196]">{mutationError(queryError)}</p>
                    <Button
                        type="button"
                        variant="outline"
                        onClick={() => {
                            void modelsQuery.refetch()
                            void nodesQuery.refetch()
                            void tenantsQuery.refetch()
                            void orchestratorsQuery.refetch()
                            void policiesQuery.refetch()
                        }}
                        className="mt-5 border-[#303C50] bg-[#111722] text-[#D8D8D8] hover:bg-[#1B2634] hover:text-white"
                    >
                        <RefreshCw className="mr-2 h-4 w-4" />
                        重新加载
                    </Button>
                </div>
            </div>
        )
    }

    return (
        <div className="flex h-full min-h-0 flex-col overflow-hidden bg-[#05070A] text-[#E8EEF7]">
            <header className="shrink-0 border-b border-white/[0.07] bg-[#070A0F] px-5 pb-3 pt-4">
                <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
                    <div className="flex items-start gap-3">
                        <div className="mt-0.5 flex h-9 w-9 items-center justify-center rounded-lg border border-[#263244] bg-[#111]">
                            <Settings2 className="h-4 w-4 text-[#8CB8F8]" />
                        </div>
                        <div>
                            <div className="flex items-center gap-2">
                                <h1 className="text-lg font-semibold tracking-tight text-[#F5F5F5]">配置中心</h1>
                                <span className="rounded-full border border-[#263244] bg-[#121212] px-2 py-0.5 text-[10px] tabular-nums text-[#747474]">
                                    {totalResources} 项资源
                                </span>
                            </div>
                            <p className="mt-1 text-xs leading-5 text-[#748196]">
                                集中定义模型、计算节点与租户策略；修改通过 Backend 写入 Kubernetes。
                            </p>
                        </div>
                    </div>

                    <div className="flex items-center gap-2 text-[11px] text-[#748196]">
                        {readOnly ? (
                            <span className="flex items-center gap-1.5 rounded-full border border-[#7D8FFF]/20 bg-[#7D8FFF]/[0.06] px-2.5 py-1.5 text-[#AEB9FF]">
                                <History className="h-3 w-3" />
                                历史只读
                            </span>
                        ) : (
                            <span className={`flex items-center gap-1.5 rounded-full border px-2.5 py-1.5 ${connectionMeta[connectionStatus].badge}`}>
                                <span className={`h-1.5 w-1.5 rounded-full ${connectionMeta[connectionStatus].dot}`} />
                                {connectionMeta[connectionStatus].label}
                            </span>
                        )}
                        <span className="hidden rounded-full border border-[#263244] bg-[#101010] px-2.5 py-1.5 sm:inline-flex">
                            Kubernetes 数据源
                        </span>
                    </div>
                </div>

                {readOnly && (
                    <div className="mt-3 flex flex-col gap-2 rounded-xl border border-[#7D8FFF]/15 bg-[#7D8FFF]/[0.045] px-3.5 py-2.5 sm:flex-row sm:items-center sm:justify-between">
                        <div className="min-w-0 text-[10px] leading-5 text-[#8997C8]">
                            <span className="font-medium text-[#AEB9FF]">历史切面对照</span>
                            <span className="mx-2 text-[#4F5C83]">·</span>
                            <span className="font-mono">{formatUtcTimestamp(workspace.effectiveAt, true)} UTC</span>
                            <span className="ml-2 hidden text-[#66739A] md:inline">当前配置仅供只读参照，不会写回历史。</span>
                        </div>
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={returnToLatest}
                            className="h-7 shrink-0 gap-1.5 px-2 text-[10px] text-[#AEB9FF] hover:bg-[#7D8FFF]/10 hover:text-white"
                        >
                            <RotateCcw className="h-3 w-3" />
                            回到最新
                        </Button>
                    </div>
                )}

                <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as ConfigTab)} className="mt-4">
                    <TabsList className="h-9 w-full justify-start gap-1 rounded-lg border border-[#202B3A] bg-[#0B1018] p-1 sm:w-auto">
                        <TabsTrigger
                            value="models"
                            className="h-7 gap-2 rounded-md px-3 text-xs text-[#748196] transition data-[state=active]:bg-[#202B3A] data-[state=active]:text-white data-[state=active]:shadow-none"
                        >
                            <BrainCircuit className="h-3.5 w-3.5" />
                            模型
                            <span className="rounded bg-black/30 px-1.5 py-0.5 text-[10px] tabular-nums text-[#8A8A8A]">{models.length}</span>
                        </TabsTrigger>
                        <TabsTrigger
                            value="nodes"
                            className="h-7 gap-2 rounded-md px-3 text-xs text-[#748196] transition data-[state=active]:bg-[#202B3A] data-[state=active]:text-white data-[state=active]:shadow-none"
                        >
                            <Server className="h-3.5 w-3.5" />
                            节点
                            <span className="rounded bg-black/30 px-1.5 py-0.5 text-[10px] tabular-nums text-[#8A8A8A]">{nodes.length}</span>
                        </TabsTrigger>
                        <TabsTrigger
                            value="tenants"
                            className="h-7 gap-2 rounded-md px-3 text-xs text-[#748196] transition data-[state=active]:bg-[#202B3A] data-[state=active]:text-white data-[state=active]:shadow-none"
                        >
                            <Users className="h-3.5 w-3.5" />
                            租户
                            <span className="rounded bg-black/30 px-1.5 py-0.5 text-[10px] tabular-nums text-[#8A8A8A]">{tenants.length}</span>
                        </TabsTrigger>
                        <TabsTrigger
                            value="orchestrators"
                            className="h-7 gap-2 rounded-md px-3 text-xs text-[#748196] transition data-[state=active]:bg-[#202B3A] data-[state=active]:text-white data-[state=active]:shadow-none"
                        >
                            <SlidersHorizontal className="h-3.5 w-3.5" />
                            编排策略
                            <span className="rounded bg-black/30 px-1.5 py-0.5 text-[10px] tabular-nums text-[#8A8A8A]">{orchestrators.length}</span>
                        </TabsTrigger>
                        <TabsTrigger
                            value="policies"
                            className="h-7 gap-2 rounded-md px-3 text-xs text-[#748196] transition data-[state=active]:bg-[#202B3A] data-[state=active]:text-white data-[state=active]:shadow-none"
                        >
                            <ShieldCheck className="h-3.5 w-3.5" />
                            策略
                            <span className="rounded bg-black/30 px-1.5 py-0.5 text-[10px] tabular-nums text-[#8A8A8A]">{policies.length}</span>
                        </TabsTrigger>
                    </TabsList>
                </Tabs>
            </header>

            <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as ConfigTab)} className="min-h-0 flex-1">
                <TabsContent value="models" forceMount className="m-0 h-full min-h-0 data-[state=inactive]:hidden">
                    <ConfigTabPanel<Model, ModelFormValues>
                        data={models}
                        selectedItem={modelSelection.selectedItem}
                        onSelect={(item) => modelSelection.setSelectedName(item.name)}
                        onRename={(resource) => openRename({ type: 'model', resource })}
                        onDelete={(name) => deleteModel.mutateAsync(name)}
                        selectedIds={selectedModels}
                        onSelectionChange={setSelectedModels}
                        TableComponent={ModelTable}
                        FormComponent={ModelForm}
                        getFormValues={modelFormValues}
                        typeLabel="模型"
                        listTitle="模型资源"
                        listDescription="推理模型、资源需求与性能画像"
                        detailDescription="模型参数"
                        resourceIcon={<BrainCircuit className="h-4 w-4" />}
                        onCreate={() => openCreate('model')}
                        onCreateFromTemplate={() => openCreateFromTemplate('model')}
                        onBatchDelete={() => openBatchDelete('model')}
                        formSubmit={saveModel}
                        readOnly={readOnly}
                    />
                </TabsContent>

                <TabsContent value="nodes" forceMount className="m-0 h-full min-h-0 data-[state=inactive]:hidden">
                    <ConfigTabPanel<Node, NodeFormValues>
                        data={nodes}
                        selectedItem={nodeSelection.selectedItem}
                        onSelect={(item) => nodeSelection.setSelectedName(item.name)}
                        onRename={(resource) => openRename({ type: 'node', resource })}
                        onDelete={(name) => deleteNode.mutateAsync(name)}
                        selectedIds={selectedNodes}
                        onSelectionChange={setSelectedNodes}
                        TableComponent={NodeTable}
                        FormComponent={NodeForm}
                        getFormValues={nodeFormValues}
                        typeLabel="节点"
                        listTitle="计算节点"
                        listDescription="本地模拟中的可调度算力资源"
                        detailDescription="容量参数"
                        resourceIcon={<Server className="h-4 w-4" />}
                        onCreate={() => openCreate('node')}
                        onCreateFromTemplate={() => openCreateFromTemplate('node')}
                        onBatchDelete={() => openBatchDelete('node')}
                        formSubmit={saveNode}
                        readOnly={readOnly}
                    />
                </TabsContent>

                <TabsContent value="tenants" forceMount className="m-0 h-full min-h-0 data-[state=inactive]:hidden">
                    <ConfigTabPanel<Tenant, TenantFormValues>
                        data={tenants}
                        selectedItem={tenantSelection.selectedItem}
                        onSelect={(item) => tenantSelection.setSelectedName(item.name)}
                        onRename={(resource) => openRename({ type: 'tenant', resource })}
                        onDelete={(name) => deleteTenant.mutateAsync(name)}
                        selectedIds={selectedTenants}
                        onSelectionChange={setSelectedTenants}
                        TableComponent={TenantTable}
                        FormComponent={TenantForm}
                        getFormValues={tenantFormValues}
                        typeLabel="租户"
                        listTitle="业务租户"
                        listDescription="优先级、基准流量与弹性阈值"
                        detailDescription="调度策略"
                        resourceIcon={<Users className="h-4 w-4" />}
                        onCreate={() => openCreate('tenant')}
                        onCreateFromTemplate={() => openCreateFromTemplate('tenant')}
                        onBatchDelete={() => openBatchDelete('tenant')}
                        formSubmit={saveTenant}
                        readOnly={readOnly}
                    />
                </TabsContent>

                <TabsContent value="orchestrators" forceMount className="m-0 h-full min-h-0 data-[state=inactive]:hidden">
                    <ConfigTabPanel<Orchestrator, OrchestratorFormValues>
                        data={orchestrators}
                        selectedItem={orchestratorSelection.selectedItem}
                        onSelect={(item) => orchestratorSelection.setSelectedName(item.name)}
                        onDelete={(name) => deleteOrchestrator.mutateAsync(name)}
                        selectedIds={selectedOrchestrators}
                        onSelectionChange={setSelectedOrchestrators}
                        TableComponent={OrchestratorTable}
                        FormComponent={OrchestratorForm}
                        getFormValues={orchestratorFormValues}
                        typeLabel="编排策略"
                        listTitle="租户编排策略"
                        listDescription="扩缩容冷却、副本范围与缩容行为"
                        detailDescription="策略参数"
                        resourceIcon={<SlidersHorizontal className="h-4 w-4" />}
                        onCreate={() => openCreate('orchestrator')}
                        onCreateFromTemplate={() => openCreateFromTemplate('orchestrator')}
                        onBatchDelete={() => openBatchDelete('orchestrator')}
                        formSubmit={saveOrchestrator}
                        readOnly={readOnly}
                    />
                </TabsContent>

                <TabsContent value="policies" forceMount className="m-0 h-full min-h-0 data-[state=inactive]:hidden">
                    <ConfigTabPanel<Policy, PolicyFormValues>
                        data={policies}
                        selectedItem={policySelection.selectedItem}
                        onSelect={(item) => policySelection.setSelectedName(item.name)}
                        onDelete={deletePolicyByName}
                        selectedIds={selectedPolicies}
                        onSelectionChange={setSelectedPolicies}
                        TableComponent={PolicyTable}
                        FormComponent={PolicyForm}
                        getFormValues={policyFormValues}
                        typeLabel="策略"
                        listTitle="访问策略"
                        listDescription="租户、模型与节点的 Allow / Deny 关系，Controller 据此拉起 Simulator"
                        detailDescription="策略参数"
                        resourceIcon={<ShieldCheck className="h-4 w-4" />}
                        onCreate={() => openCreatePolicy('tenantModel')}
                        onBatchDelete={openBatchDeletePolicy}
                        formSubmit={savePolicy}
                        readOnly={readOnly}
                    />
                </TabsContent>
            </Tabs>

            <TemplateLibraryDialog
                open={createFromTemplateOpen && createType === 'model'}
                onOpenChange={setCreateFromTemplateOpen}
                templates={modelTemplates}
                typeLabel="模型"
                pickMode
                onLoad={(template) => {
                    applyCreateTemplate({ type: 'model', data: template.data })
                }}
                onDelete={removeModelTemplate}
                getPreview={getModelPreview}
            />

            <TemplateLibraryDialog
                open={createFromTemplateOpen && createType === 'node'}
                onOpenChange={setCreateFromTemplateOpen}
                templates={nodeTemplates}
                typeLabel="节点"
                pickMode
                onLoad={(template) => {
                    applyCreateTemplate({ type: 'node', data: template.data })
                }}
                onDelete={removeNodeTemplate}
                getPreview={getNodePreview}
            />

            <TemplateLibraryDialog
                open={createFromTemplateOpen && createType === 'tenant'}
                onOpenChange={setCreateFromTemplateOpen}
                templates={tenantTemplates}
                typeLabel="租户"
                pickMode
                onLoad={(template) => {
                    applyCreateTemplate({ type: 'tenant', data: template.data })
                }}
                onDelete={removeTenantTemplate}
                getPreview={getTenantPreview}
            />

            <TemplateLibraryDialog
                open={createFromTemplateOpen && createType === 'orchestrator'}
                onOpenChange={setCreateFromTemplateOpen}
                templates={orchestratorTemplates}
                typeLabel="编排策略"
                pickMode
                onLoad={(template) => {
                    applyCreateTemplate({ type: 'orchestrator', data: template.data })
                }}
                onDelete={removeOrchestratorTemplate}
                getPreview={getOrchestratorPreview}
            />

            <PolicyCreateDialog
                open={policyCreateOpen}
                onOpenChange={(open) => {
                    setPolicyCreateOpen(open)
                    if (!open) setPolicyCreateError('')
                }}
                kind={policyCreateKind}
                onKindChange={(kind) => {
                    setPolicyCreateKind(kind)
                    setPolicyTenantName('')
                    setPolicyModelName('')
                    setPolicyNodeName('')
                    setPolicyCreateError('')
                }}
                tenantName={policyTenantName}
                onTenantNameChange={(value) => {
                    setPolicyTenantName(value)
                    setPolicyCreateError('')
                }}
                modelName={policyModelName}
                onModelNameChange={(value) => {
                    setPolicyModelName(value)
                    setPolicyCreateError('')
                }}
                nodeName={policyNodeName}
                onNodeNameChange={(value) => {
                    setPolicyNodeName(value)
                    setPolicyCreateError('')
                }}
                effect={policyEffect}
                onEffectChange={setPolicyEffect}
                identifierPreview={policyCreateOpen ? policyIdentifierPreview : ''}
                tenants={tenants}
                models={models}
                nodes={nodes}
                pending={createPolicy.isPending}
                error={policyCreateError}
                onConfirm={() => void confirmCreatePolicy()}
            />

            <CreateDialog
                open={createOpen}
                onOpenChange={(open) => {
                    setCreateOpen(open)
                    if (!open) setCreateError('')
                }}
                type={createType}
                value={newName}
                onValueChange={(value) => {
                    setNewName(value)
                    setCreateError('')
                }}
                identifierPreview={newName.trim() ? identifierPreview : ''}
                nameLabel={createType === 'orchestrator' ? '关联租户名称' : '显示名称'}
                pending={createPending}
                error={createError}
                onConfirm={() => void confirmCreate()}
            />

            <RenameDialog
                open={Boolean(renameTarget)}
                onOpenChange={(open) => {
                    if (!open) {
                        setRenameTarget(null)
                        setRenameError('')
                    }
                }}
                typeLabel={renameTarget ? resourceLabels[renameTarget.type] : ''}
                resourceId={renameTarget?.resource.name ?? ''}
                value={renameValue}
                onValueChange={(value) => {
                    setRenameValue(value)
                    setRenameError('')
                }}
                pending={renamePending}
                error={renameError}
                onConfirm={() => void confirmRename()}
            />

            <BatchDeleteDialog
                open={Boolean(batchDeleteType)}
                onOpenChange={(open) => {
                    if (!open) {
                        setBatchDeleteType(null)
                        setBatchDeleteError('')
                    }
                }}
                typeLabel={batchDeleteType ? resourceLabels[batchDeleteType] : ''}
                count={batchDeleteCount}
                pending={batchDeletePending}
                error={batchDeleteError}
                onConfirm={() => void confirmBatchDelete()}
            />

            <BatchDeleteDialog
                open={policyBatchDeleteOpen}
                onOpenChange={(open) => {
                    if (!open) {
                        setPolicyBatchDeleteOpen(false)
                        setPolicyBatchDeleteError('')
                    }
                }}
                typeLabel="策略"
                count={selectedPolicies.length}
                pending={deletePolicy.isPending}
                error={policyBatchDeleteError}
                onConfirm={() => void confirmPolicyBatchDelete()}
            />
        </div>
    )
}
