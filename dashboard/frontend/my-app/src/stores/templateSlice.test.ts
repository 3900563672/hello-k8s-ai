import { beforeEach, describe, expect, it } from 'vitest'
import { useTemplateStore } from '@/stores/templateSlice'

describe('templateSlice', () => {
    beforeEach(() => {
        useTemplateStore.setState(useTemplateStore.getInitialState())
    })

    it('初始加载四类预置模板', () => {
        const state = useTemplateStore.getState()
        expect(state.modelTemplates.length).toBeGreaterThan(0)
        expect(state.nodeTemplates.length).toBeGreaterThan(0)
        expect(state.tenantTemplates.length).toBeGreaterThan(0)
        expect(state.orchestratorTemplates.length).toBeGreaterThan(0)
    })

    it('addModelTemplate 生成模板并追加', () => {
        useTemplateStore.getState().addModelTemplate(' 我的模型 ', {
            displayName: 'm',
            gpuUnits: 2,
            maxConcurrency: 4,
            absoluteScore: 80,
            coldStartMs: 100,
            performance: { prefillBaseMs: 10, prefillPerTokenUs: 5, decodePerTokenMs: 3 },
        })
        const templates = useTemplateStore.getState().modelTemplates
        const created = templates[templates.length - 1]
        expect(created.name).toBe('我的模型')
        expect(created.id).toMatch(/^template-/)
        expect(created.data).toEqual({
            displayName: 'm',
            gpuUnits: 2,
            maxConcurrency: 4,
            absoluteScore: 80,
            coldStartMs: 100,
            performance: { prefillBaseMs: 10, prefillPerTokenUs: 5, decodePerTokenMs: 3 },
        })
        expect(Number.isNaN(Date.parse(created.createdAt))).toBe(false)
    })

    it('addOrchestratorTemplate 与 removeOrchestratorTemplate 配对', () => {
        useTemplateStore.getState().addOrchestratorTemplate('编排模板', {
            tenantName: 't',
            scaleUpCooldownSeconds: 30,
            scaleDownCooldownSeconds: 30,
            allowScaleToZero: true,
            minReplicas: 1,
            maxReplicas: 3,
            maxScaleUpBatch: 1,
        })
        const templates = useTemplateStore.getState().orchestratorTemplates
        const created = templates[templates.length - 1]
        expect(created.name).toBe('编排模板')
        useTemplateStore.getState().removeOrchestratorTemplate(created.id)
        expect(
            useTemplateStore.getState().orchestratorTemplates.some((item) => item.id === created.id),
        ).toBe(false)
    })

    it('removeModelTemplate 只删除指定模板', () => {
        const before = useTemplateStore.getState().modelTemplates
        useTemplateStore.getState().removeModelTemplate(before[0].id)
        const after = useTemplateStore.getState().modelTemplates
        expect(after).toHaveLength(before.length - 1)
        expect(after.some((item) => item.id === before[0].id)).toBe(false)
    })
})
