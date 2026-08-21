import { describe, expect, it } from 'vitest'
import { nodeSchema, getNodePreview } from '@/lib/validations/node.schema'
import { policySchema, getPolicyPreview } from '@/lib/validations/policy.schema'
import { orchestratorSchema, getOrchestratorPreview } from '@/lib/validations/orchestrator.schema'
import { tenantSchema, getTenantPreview } from '@/lib/validations/tenant.schema'
import { modelSchema, getModelPreview } from '@/lib/validations/model.schema'

describe('validations 补全', () => {
    it('getNodePreview 输出预览键值', () => {
        const data = nodeSchema.parse({ displayName: '节点', gpu: 8, maxConcurrency: 16 })
        expect(getNodePreview(data)).toEqual([
            { key: '总显存', value: 8, unit: 'G' },
            { key: '最大并发', value: 16 },
        ])
    })

    it('policySchema tenantNode 分支要求租户与节点', () => {
        const missing = policySchema.safeParse({ kind: 'tenantNode', tenantName: '', modelName: '', nodeName: '', effect: 'Allow' })
        expect(missing.success).toBe(false)
        if (!missing.success) {
            const paths = missing.error.issues.map((issue) => issue.path.join('.'))
            expect(paths).toContain('tenantName')
            expect(paths).toContain('nodeName')
        }
        const ok = policySchema.safeParse({ kind: 'tenantNode', tenantName: 't', modelName: '', nodeName: 'n', effect: 'Deny' })
        expect(ok.success).toBe(true)
    })

    it('policySchema modelNode 分支只要求模型与节点', () => {
        const ok = policySchema.safeParse({ kind: 'modelNode', tenantName: '', modelName: 'm', nodeName: 'n', effect: 'Allow' })
        expect(ok.success).toBe(true)
    })

    it('getPolicyPreview 按 kind 组合展示名', () => {
        const tm = getPolicyPreview({ kind: 'tenantModel', tenantName: 't', modelName: 'm', nodeName: '', effect: 'Allow' })
        expect(tm.map((item) => item.key)).toContain('租户')
        const mn = getPolicyPreview({ kind: 'modelNode', tenantName: '', modelName: 'm', nodeName: 'n', effect: 'Allow' })
        expect(mn.map((item) => item.key)).toContain('模型')
        expect(mn.map((item) => item.key)).toContain('节点')
        expect(mn.some((item) => item.key === '租户')).toBe(false)
    })

    it('getOrchestratorPreview / getTenantPreview / getModelPreview 输出预览', () => {
        const orch = orchestratorSchema.parse({
            tenantName: 't', scaleUpCooldownSeconds: 30, scaleDownCooldownSeconds: 60,
            allowScaleToZero: true, minReplicas: 1, maxReplicas: 3, maxScaleUpBatch: 2,
        })
        expect(getOrchestratorPreview(orch).length).toBeGreaterThan(0)
        const tenant = tenantSchema.parse({
            displayName: '租户', priority: 'P1', qps: 100,
            ttftThresholdMs: 2000, queueThreshold: 50,
            ttftScaleDownThresholdMs: 1000, queueScaleDownThreshold: 20,
        })
        expect(getTenantPreview(tenant).length).toBeGreaterThan(0)
        const model = modelSchema.parse({
            displayName: '模型', gpuUnits: 2, maxConcurrency: 4, absoluteScore: 80,
            coldStartMs: 100,
            performance: { prefillBaseMs: 10, prefillPerTokenUs: 5, decodePerTokenMs: 3 },
        })
        expect(getModelPreview(model).length).toBeGreaterThan(0)
    })
})