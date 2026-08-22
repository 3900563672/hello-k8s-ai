import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ConfigPage } from '@/components/features/config/ConfigPage'
import { useTimeStore } from '@/stores/timeSlice'
import type { Model, Node, Orchestrator, Policy, Tenant } from '@/types/config.types'

const h = vi.hoisted(() => {
    const query = () => ({
        data: undefined as unknown,
        isLoading: false,
        isError: false,
        error: undefined as { message: string } | undefined,
        refetch: vi.fn(),
    })
    const mutation = () => ({ isPending: false, mutateAsync: vi.fn().mockResolvedValue(undefined) })
    return {
        queries: {
            models: query(),
            nodes: query(),
            tenants: query(),
            orchestrators: query(),
            policies: query(),
        },
        mutations: {
            createModel: mutation(),
            updateModel: mutation(),
            deleteModel: mutation(),
            deleteModels: mutation(),
            createNode: mutation(),
            updateNode: mutation(),
            deleteNode: mutation(),
            deleteNodes: mutation(),
            createTenant: mutation(),
            updateTenant: mutation(),
            deleteTenant: mutation(),
            deleteTenants: mutation(),
            createOrchestrator: mutation(),
            updateOrchestrator: mutation(),
            deleteOrchestrator: mutation(),
            deleteOrchestrators: mutation(),
            createPolicy: mutation(),
            updatePolicy: mutation(),
            deletePolicy: mutation(),
        },
        workspace: {
            isHistorical: false,
            cluster: { connectionStatus: 'connected', workers: [] },
            effectiveAt: '2026-08-20T00:00:00.000Z',
        },
        modelTemplates: [] as Array<{ id: string; name: string; data: Record<string, unknown> }>,
        removeTemplate: vi.fn(),
    }
})

vi.mock('@/api/queries/configQueries', () => ({
    useModels: () => h.queries.models,
    useNodes: () => h.queries.nodes,
    useTenants: () => h.queries.tenants,
    useOrchestrators: () => h.queries.orchestrators,
    usePolicies: () => h.queries.policies,
    useCreateModel: () => h.mutations.createModel,
    useUpdateModel: () => h.mutations.updateModel,
    useDeleteModel: () => h.mutations.deleteModel,
    useDeleteModels: () => h.mutations.deleteModels,
    useCreateNode: () => h.mutations.createNode,
    useUpdateNode: () => h.mutations.updateNode,
    useDeleteNode: () => h.mutations.deleteNode,
    useDeleteNodes: () => h.mutations.deleteNodes,
    useCreateTenant: () => h.mutations.createTenant,
    useUpdateTenant: () => h.mutations.updateTenant,
    useDeleteTenant: () => h.mutations.deleteTenant,
    useDeleteTenants: () => h.mutations.deleteTenants,
    useCreateOrchestrator: () => h.mutations.createOrchestrator,
    useUpdateOrchestrator: () => h.mutations.updateOrchestrator,
    useDeleteOrchestrator: () => h.mutations.deleteOrchestrator,
    useDeleteOrchestrators: () => h.mutations.deleteOrchestrators,
    useCreatePolicy: () => h.mutations.createPolicy,
    useUpdatePolicy: () => h.mutations.updatePolicy,
    useDeletePolicy: () => h.mutations.deletePolicy,
}))

vi.mock('@/hooks/useWorkspaceContext', () => ({
    useWorkspaceContext: () => h.workspace,
}))

vi.mock('@/stores/templateSlice', () => ({
    useTemplateStore: () => ({
        modelTemplates: h.modelTemplates,
        nodeTemplates: [],
        tenantTemplates: [],
        orchestratorTemplates: [],
        removeModelTemplate: h.removeTemplate,
        removeNodeTemplate: vi.fn(),
        removeTenantTemplate: vi.fn(),
        removeOrchestratorTemplate: vi.fn(),
    }),
}))

// ---- 子组件 stub：保留交互入口，触发 ConfigPage 自身的编排逻辑 ----
vi.mock('@/components/features/config/components/ConfigTabPanel', () => ({
    ConfigTabPanel: (props: any) => (
        <div data-testid={`tab-panel-${props.typeLabel}`}>
            <span>{props.data.map((item: { name: string }) => item.name).join(',')}</span>
            <span>selected:{props.selectedItem?.name ?? 'none'}</span>
            {!props.readOnly && (
                <>
                    <button type="button" onClick={() => props.onCreate?.()}>create-{props.typeLabel}</button>
                    <button type="button" onClick={() => props.onCreateFromTemplate?.()}>template-{props.typeLabel}</button>
                    <button type="button" onClick={() => props.onSelectionChange?.(props.data.map((item: { name: string }) => item.name))}>select-all-{props.typeLabel}</button>
                    <button type="button" onClick={() => props.onBatchDelete?.()}>batch-{props.typeLabel}</button>
                    <button type="button" onClick={() => props.onRename?.(props.selectedItem)}>rename-{props.typeLabel}</button>
                    <button type="button" onClick={() => props.onDelete?.(props.selectedItem?.name)}>delete-{props.typeLabel}</button>
                    <button type="button" onClick={() => props.formSubmit?.({ displayName: '保存后的名字' })}>save-{props.typeLabel}</button>
                </>
            )}
        </div>
    ),
}))

const dialogShell = (testId: string, props: any, extra?: React.ReactNode) =>
    props.open ? (
        <div data-testid={testId}>
            {extra}
            <button type="button" onClick={props.onConfirm}>confirm</button>
            <button type="button" onClick={() => props.onOpenChange(false)}>close</button>
        </div>
    ) : null

vi.mock('@/components/shared/dialogs/CreateDialog', () => ({
    CreateDialog: (props: any) => dialogShell('create-dialog', props, (
        <>
            <span>type:{props.type}</span>
            <span>value:{props.value}</span>
            <span>preview:{props.identifierPreview}</span>
            <span>error:{props.error ?? ''}</span>
            <button type="button" onClick={() => props.onValueChange('模型X')}>set-name</button>
        </>
    )),
}))

vi.mock('@/components/shared/dialogs/RenameDialog', () => ({
    RenameDialog: (props: any) => dialogShell('rename-dialog', props, (
        <>
            <span>rename-value:{props.value}</span>
            <button type="button" onClick={() => props.onValueChange('新显示名')}>set-rename</button>
        </>
    )),
}))

vi.mock('@/components/shared/dialogs/BatchDeleteDialog', () => ({
    BatchDeleteDialog: (props: any) => dialogShell('batch-dialog', props, (
        <span>count:{props.count}</span>
    )),
}))

vi.mock('@/components/shared/dialogs/PolicyCreateDialog', () => ({
    PolicyCreateDialog: (props: any) => dialogShell('policy-dialog', props, (
        <>
            <span>kind:{props.kind}</span>
            <span>preview:{props.identifierPreview}</span>
            <span>error:{props.error ?? ''}</span>
            <button type="button" onClick={() => props.onTenantNameChange('tenant-a')}>set-tenant</button>
            <button type="button" onClick={() => props.onModelNameChange('model-a')}>set-model</button>
        </>
    )),
}))

vi.mock('@/components/shared/dialogs/TemplateLibraryDialog', () => ({
    TemplateLibraryDialog: (props: any) => props.open ? (
        <div data-testid="template-dialog">
            <span>templates:{props.templates.length}</span>
            <button
                type="button"
                onClick={() => props.onLoad?.({ id: 't1', name: '模板1', data: { displayName: '模板模型', gpuUnits: 2, maxConcurrency: 128, absoluteScore: 90, coldStartMs: 50, performance: { prefillBaseMs: 10, prefillPerTokenUs: 1, decodePerTokenMs: 5 } } })}
            >
                load-template
            </button>
        </div>
    ) : null,
}))

// tables/forms 引用由 TabPanel stub 消化，但仍需解析模块——mock 为 null
vi.mock('@/components/features/config/tables/ModelTable', () => ({ ModelTable: () => null }))
vi.mock('@/components/features/config/tables/NodeTable', () => ({ NodeTable: () => null }))
vi.mock('@/components/features/config/tables/OrchestratorTable', () => ({ OrchestratorTable: () => null }))
vi.mock('@/components/features/config/tables/PolicyTable', () => ({ PolicyTable: () => null }))
vi.mock('@/components/features/config/tables/TenantTable', () => ({ TenantTable: () => null }))
vi.mock('@/components/features/config/forms/ModelForm', () => ({ ModelForm: () => null }))
vi.mock('@/components/features/config/forms/NodeForm', () => ({ NodeForm: () => null }))
vi.mock('@/components/features/config/forms/OrchestratorForm', () => ({ OrchestratorForm: () => null }))
vi.mock('@/components/features/config/forms/PolicyForm', () => ({ PolicyForm: () => null }))
vi.mock('@/components/features/config/forms/TenantForm', () => ({ TenantForm: () => null }))

const modelA: Model = {
    name: 'model-a',
    displayName: '模型A',
    gpuUnits: 1,
    maxConcurrency: 64,
    absoluteScore: 100,
    coldStartMs: 100,
    performance: { prefillBaseMs: 10, prefillPerTokenUs: 1, decodePerTokenMs: 5 },
}
const nodeA: Node = { name: 'node-a', displayName: '节点A', gpu: 1, maxConcurrency: 64 }
const tenantA: Tenant = {
    name: 'tenant-a',
    displayName: '租户A',
    priority: 'P1',
    qps: 30,
    ttftThresholdMs: 300,
    queueThreshold: 50,
    ttftScaleDownThresholdMs: 200,
    queueScaleDownThreshold: 10,
}
const orchestratorA: Orchestrator = {
    name: 'orch-a',
    displayName: '租户A',
    tenantRef: { name: 'tenant-a' },
    scaleUpCooldownSeconds: 60,
    scaleDownCooldownSeconds: 120,
    allowScaleToZero: false,
    minReplicas: 1,
    maxReplicas: 10,
    maxScaleUpBatch: 4,
}
const policyA: Policy = {
    name: 'tenant-a-model-a',
    displayName: 'tenant-a → model-a',
    kind: 'tenantModel',
    tenantRef: { name: 'tenant-a' },
    modelRef: { name: 'model-a' },
    effect: 'Allow',
}

function loadAll() {
    h.queries.models.data = [modelA]
    h.queries.nodes.data = [nodeA]
    h.queries.tenants.data = [tenantA]
    h.queries.orchestrators.data = [orchestratorA]
    h.queries.policies.data = [policyA]
}

describe('ConfigPage', () => {
    beforeEach(() => {
        useTimeStore.setState({ mode: 'latest', timestamp: new Date(0).toISOString(), selectedSnapshotId: null, revision: 0, snapshots: [] })
        h.workspace.isHistorical = false
        h.workspace.cluster = { connectionStatus: 'connected', workers: [] }
        h.workspace.effectiveAt = '2026-08-20T00:00:00.000Z'
        loadAll()
        for (const q of Object.values(h.queries)) {
            q.isLoading = false
            q.isError = false
            q.error = undefined
            q.refetch.mockClear()
        }
        for (const m of Object.values(h.mutations)) {
            m.isPending = false
            m.mutateAsync.mockClear()
            m.mutateAsync.mockResolvedValue(undefined)
        }
        h.modelTemplates = []
        h.removeTemplate.mockClear()
    })

    afterEach(() => cleanup())

    it('加载态：显示骨架屏', () => {
        h.queries.models.isLoading = true
        render(<ConfigPage />)
        expect(document.querySelectorAll('.animate-pulse').length).toBeGreaterThan(0)
    })

    it('错误态：显示错误信息，重新加载触发全部 refetch', async () => {
        const user = userEvent.setup()
        h.queries.nodes.isError = true
        h.queries.nodes.error = new Error('config api down')
        render(<ConfigPage />)
        expect(screen.getByText('无法读取本地配置')).toBeInTheDocument()
        expect(screen.getByText('config api down')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: /重新加载/ }))
        expect(h.queries.models.refetch).toHaveBeenCalledTimes(1)
        expect(h.queries.nodes.refetch).toHaveBeenCalledTimes(1)
        expect(h.queries.policies.refetch).toHaveBeenCalledTimes(1)
    })

    it('正常渲染：标题、资源计数、连接徽标与五个 Tab', () => {
        render(<ConfigPage />)
        expect(screen.getByText('配置中心')).toBeInTheDocument()
        expect(screen.getByText('5 项资源')).toBeInTheDocument()
        expect(screen.getByText('Backend 已连接')).toBeInTheDocument()
        expect(screen.getByRole('tab', { name: /模型/ })).toBeInTheDocument()
        expect(screen.getByRole('tab', { name: /节点/ })).toBeInTheDocument()
        expect(screen.getByRole('tab', { name: /租户/ })).toBeInTheDocument()
        expect(screen.getByRole('tab', { name: /编排策略/ })).toBeInTheDocument()
        expect(screen.getByRole('tab', { name: /^策略/ })).toBeInTheDocument()
        // 模型面板（默认 tab）显示数据与选中项
        expect(screen.getByTestId('tab-panel-模型')).toBeInTheDocument()
        expect(screen.getByText('selected:model-a')).toBeInTheDocument()
    })

    it('Tab 切换：切到节点面板显示节点数据', async () => {
        const user = userEvent.setup()
        render(<ConfigPage />)
        await user.click(screen.getByRole('tab', { name: /节点/ }))
        expect(screen.getByTestId('tab-panel-节点')).toBeInTheDocument()
        expect(screen.getByText('selected:node-a')).toBeInTheDocument()
        await user.click(screen.getByRole('tab', { name: /^策略/ }))
        expect(screen.getByText('selected:tenant-a-model-a')).toBeInTheDocument()
    })

    it('创建模型：弹窗预填预览，确认调用 createModel', async () => {
        const user = userEvent.setup()
        render(<ConfigPage />)
        await user.click(screen.getByRole('button', { name: 'create-模型' }))
        expect(screen.getByTestId('create-dialog')).toBeInTheDocument()
        expect(screen.getByText('type:model')).toBeInTheDocument()
        // 输入名称后 identifierPreview 生成（"模型X" → 归一化 → x）
        await user.click(screen.getByRole('button', { name: 'set-name' }))
        expect(screen.getByText('value:模型X')).toBeInTheDocument()
        expect(screen.getByText('preview:x')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: 'confirm' }))
        expect(h.mutations.createModel.mutateAsync).toHaveBeenCalledTimes(1)
        const arg = h.mutations.createModel.mutateAsync.mock.calls[0][0] as Model
        expect(arg.name).toBe('x')
        expect(arg.displayName).toBe('模型X')
        expect(screen.queryByTestId('create-dialog')).not.toBeInTheDocument()
    })

    it('创建模型冲突时 identifier 加后缀', async () => {
        const user = userEvent.setup()
        h.queries.models.data = [{ ...modelA, name: 'x' }]
        render(<ConfigPage />)
        await user.click(screen.getByRole('button', { name: 'create-模型' }))
        await user.click(screen.getByRole('button', { name: 'set-name' }))
        expect(screen.getByText('preview:x-2')).toBeInTheDocument()
    })

    it('创建编排策略：未找到租户时报错，找到后调用 createOrchestrator', async () => {
        const user = userEvent.setup()
        render(<ConfigPage />)
        await user.click(screen.getByRole('tab', { name: /编排策略/ }))
        await user.click(screen.getByRole('button', { name: 'create-编排策略' }))
        expect(screen.getByTestId('create-dialog')).toBeInTheDocument()
        // 输入不存在的租户
        await user.click(screen.getByRole('button', { name: 'set-name' }))
        await user.click(screen.getByRole('button', { name: 'confirm' }))
        expect(screen.getByText(/未找到租户/)).toBeInTheDocument()
        expect(h.mutations.createOrchestrator.mutateAsync).not.toHaveBeenCalled()
        // 输入存在的租户（通过 onValueChange 直接传租户名）
        await user.click(screen.getByRole('button', { name: 'close' }))
        await user.click(screen.getByRole('button', { name: 'create-编排策略' }))
        expect(screen.getByText('type:orchestrator')).toBeInTheDocument()
    })

    it('重命名模型：确认调用 updateModel', async () => {
        const user = userEvent.setup()
        render(<ConfigPage />)
        await user.click(screen.getByRole('button', { name: 'rename-模型' }))
        expect(screen.getByTestId('rename-dialog')).toBeInTheDocument()
        expect(screen.getByText('rename-value:模型A')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: 'set-rename' }))
        await user.click(screen.getByRole('button', { name: 'confirm' }))
        expect(h.mutations.updateModel.mutateAsync).toHaveBeenCalledTimes(1)
        const arg = h.mutations.updateModel.mutateAsync.mock.calls[0][0] as Model
        expect(arg.displayName).toBe('新显示名')
        expect(screen.queryByTestId('rename-dialog')).not.toBeInTheDocument()
    })

    it('批量删除模型：确认调用 deleteModels', async () => {
        const user = userEvent.setup()
        render(<ConfigPage />)
        await user.click(screen.getByRole('button', { name: 'select-all-模型' }))
        await user.click(screen.getByRole('button', { name: 'batch-模型' }))
        expect(screen.getByTestId('batch-dialog')).toBeInTheDocument()
        expect(screen.getByText('count:1')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: 'confirm' }))
        expect(h.mutations.deleteModels.mutateAsync).toHaveBeenCalledWith(['model-a'])
        expect(screen.queryByTestId('batch-dialog')).not.toBeInTheDocument()
    })

    it('策略创建：未选对象时报错，选择后确认调用 createPolicy', async () => {
        const user = userEvent.setup()
        render(<ConfigPage />)
        await user.click(screen.getByRole('tab', { name: /^策略/ }))
        await user.click(screen.getByRole('button', { name: 'create-策略' }))
        expect(screen.getByTestId('policy-dialog')).toBeInTheDocument()
        // 未选对象 → 预览为空 → 报错
        await user.click(screen.getByRole('button', { name: 'confirm' }))
        expect(screen.getByText(/请先选择策略的引用对象/)).toBeInTheDocument()
        expect(h.mutations.createPolicy.mutateAsync).not.toHaveBeenCalled()
        // 选择 tenant + model → 预览生成 → 确认
        await user.click(screen.getByRole('button', { name: 'set-tenant' }))
        await user.click(screen.getByRole('button', { name: 'set-model' }))
        expect(screen.getByText('preview:tenant-a-model-a-2')).toBeInTheDocument() // 与已有策略重名 → 自动加后缀
        await user.click(screen.getByRole('button', { name: 'confirm' }))
        expect(h.mutations.createPolicy.mutateAsync).toHaveBeenCalledTimes(1)
        const arg = h.mutations.createPolicy.mutateAsync.mock.calls[0][0] as Policy
        expect(arg.name).toBe('tenant-a-model-a-2')
        expect(arg.effect).toBe('Allow')
        expect(screen.queryByTestId('policy-dialog')).not.toBeInTheDocument()
    })

    it('模板创建：加载模板预填创建弹窗，确认按模板数据创建', async () => {
        const user = userEvent.setup()
        render(<ConfigPage />)
        await user.click(screen.getByRole('button', { name: 'template-模型' }))
        expect(screen.getByTestId('template-dialog')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: 'load-template' }))
        // 模板库关闭，创建弹窗打开且预填显示名称
        expect(screen.queryByTestId('template-dialog')).not.toBeInTheDocument()
        expect(screen.getByTestId('create-dialog')).toBeInTheDocument()
        expect(screen.getByText('value:模板模型')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: 'confirm' }))
        expect(h.mutations.createModel.mutateAsync).toHaveBeenCalledTimes(1)
        const arg = h.mutations.createModel.mutateAsync.mock.calls[0][0] as Model
        expect(arg.gpuUnits).toBe(2)
        expect(arg.maxConcurrency).toBe(128)
        expect(arg.displayName).toBe('模板模型')
    })

    it('历史只读：显示历史徽标、Tab 面板只读、回到最新调用 store action', async () => {
        const user = userEvent.setup()
        const returnSpy = vi.spyOn(useTimeStore.getState(), 'returnToLatest').mockImplementation(() => undefined)
        h.workspace.isHistorical = true
        render(<ConfigPage />)
        expect(screen.getByText('历史只读')).toBeInTheDocument()
        // 只读面板不渲染创建按钮
        expect(screen.queryByRole('button', { name: 'create-模型' })).not.toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: /回到最新/ }))
        expect(returnSpy).toHaveBeenCalledTimes(1)
        returnSpy.mockRestore()
    })

    it('创建失败：mutateAsync 拒绝后弹窗显示错误', async () => {
        const user = userEvent.setup()
        h.mutations.createModel.mutateAsync.mockRejectedValue(new Error('写入超时'))
        render(<ConfigPage />)
        await user.click(screen.getByRole('button', { name: 'create-模型' }))
        await user.click(screen.getByRole('button', { name: 'set-name' }))
        await user.click(screen.getByRole('button', { name: 'confirm' }))
        expect(await screen.findByText(/写入超时/)).toBeInTheDocument()
        // 弹窗保持打开
        expect(screen.getByTestId('create-dialog')).toBeInTheDocument()
    })

    it('表单保存：formSubmit 调用 updateModel 合并当前选中项', async () => {
        const user = userEvent.setup()
        render(<ConfigPage />)
        await user.click(screen.getByRole('button', { name: 'save-模型' }))
        expect(h.mutations.updateModel.mutateAsync).toHaveBeenCalledTimes(1)
        const arg = h.mutations.updateModel.mutateAsync.mock.calls[0][0] as Model
        expect(arg.name).toBe('model-a')
        expect(arg.displayName).toBe('保存后的名字')
    })

    it('单个删除：onDelete 调用 deleteModel', async () => {
        const user = userEvent.setup()
        render(<ConfigPage />)
        await user.click(screen.getByRole('button', { name: 'delete-模型' }))
        expect(h.mutations.deleteModel.mutateAsync).toHaveBeenCalledWith('model-a')
    })

    it('策略保存与删除：savePolicy 合并 effect，deletePolicyByName 找对象后调用', async () => {
        const user = userEvent.setup()
        render(<ConfigPage />)
        await user.click(screen.getByRole('tab', { name: /^策略/ }))
        // 保存（selectedItem = policyA）
        await user.click(screen.getByRole('button', { name: 'save-策略' }))
        expect(h.mutations.updatePolicy.mutateAsync).toHaveBeenCalledTimes(1)
        const arg = h.mutations.updatePolicy.mutateAsync.mock.calls[0][0] as Policy
        expect(arg.name).toBe('tenant-a-model-a')
        expect(arg).toMatchObject({ tenantRef: { name: 'tenant-a' }, modelRef: { name: 'model-a' } })
        // 单个删除
        await user.click(screen.getByRole('button', { name: 'delete-策略' }))
        expect(h.mutations.deletePolicy.mutateAsync).toHaveBeenCalledTimes(1)
        expect(h.mutations.deletePolicy.mutateAsync.mock.calls[0][0]).toMatchObject({ name: 'tenant-a-model-a' })
    })

    it('连接状态徽标：disconnected 显示未连接', () => {
        h.workspace.cluster = { connectionStatus: 'disconnected', workers: [] }
        render(<ConfigPage />)
        expect(screen.getByText('Backend 未连接')).toBeInTheDocument()
    })

    it('批量删除编排策略：确认调用 deleteOrchestrators', async () => {
        const user = userEvent.setup()
        render(<ConfigPage />)
        await user.click(screen.getByRole('tab', { name: /编排策略/ }))
        await user.click(screen.getByRole('button', { name: 'select-all-编排策略' }))
        await user.click(screen.getByRole('button', { name: 'batch-编排策略' }))
        expect(screen.getByText('count:1')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: 'confirm' }))
        expect(h.mutations.deleteOrchestrators.mutateAsync).toHaveBeenCalledWith(['orch-a'])
    })
})