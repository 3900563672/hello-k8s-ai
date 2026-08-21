import { describe, expect, it } from 'vitest'
import { useTemplateStore } from '@/stores/templateSlice'

const nodeData = { displayName: '节点', gpu: 8, maxConcurrency: 16 }
const tenantData = {
    displayName: '租户', priority: 'P1' as const, qps: 100,
    ttftThresholdMs: 2000, queueThreshold: 50,
    ttftScaleDownThresholdMs: 1000, queueScaleDownThreshold: 20,
}

describe('templateSlice 节点/租户模板', () => {
    it('addNodeTemplate 生成并追加节点模板', () => {
        useTemplateStore.getState().addNodeTemplate(' 节点模板 ', nodeData)
        const templates = useTemplateStore.getState().nodeTemplates
        const created = templates[templates.length - 1]
        expect(created.name).toBe('节点模板')
        expect(created.data).toEqual(nodeData)
        expect(created.id).toMatch(/^template-/)
    })

    it('addTenantTemplate 生成并追加租户模板', () => {
        useTemplateStore.getState().addTenantTemplate('租户模板', tenantData)
        const templates = useTemplateStore.getState().tenantTemplates
        const created = templates[templates.length - 1]
        expect(created.name).toBe('租户模板')
        expect(created.data).toEqual(tenantData)
    })

    it('removeNodeTemplate / removeTenantTemplate 只删对应条目', () => {
        const nodeCount = useTemplateStore.getState().nodeTemplates.length
        const tenantCount = useTemplateStore.getState().tenantTemplates.length
        useTemplateStore.getState().addNodeTemplate('待删节点', nodeData)
        useTemplateStore.getState().addTenantTemplate('待删租户', tenantData)
        const nodeId = useTemplateStore.getState().nodeTemplates.at(-1)!.id
        const tenantId = useTemplateStore.getState().tenantTemplates.at(-1)!.id

        useTemplateStore.getState().removeNodeTemplate(nodeId)
        useTemplateStore.getState().removeTenantTemplate(tenantId)

        expect(useTemplateStore.getState().nodeTemplates).toHaveLength(nodeCount)
        expect(useTemplateStore.getState().tenantTemplates).toHaveLength(tenantCount)
    })
})