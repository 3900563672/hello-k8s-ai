import { describe, expect, it } from 'vitest'
import { modelSchema } from '@/lib/validations/model.schema'
import { nodeSchema } from '@/lib/validations/node.schema'
import { orchestratorSchema } from '@/lib/validations/orchestrator.schema'
import { policySchema } from '@/lib/validations/policy.schema'
import { tenantSchema } from '@/lib/validations/tenant.schema'

const validModel = {
    displayName: 'gpt-4o',
    gpuUnits: 2,
    maxConcurrency: 8,
    absoluteScore: 90,
    coldStartMs: 500,
    performance: {
        prefillBaseMs: 100,
        prefillPerTokenUs: 50,
        decodePerTokenMs: 20,
    },
}

describe('modelSchema', () => {
    it('接受合法模型参数', () => {
        expect(modelSchema.safeParse(validModel).success).toBe(true)
    })

    it('拒绝空名称 / 负数 / 非整数 / 非有限值', () => {
        expect(modelSchema.safeParse({ ...validModel, displayName: '  ' }).success).toBe(false)
        expect(modelSchema.safeParse({ ...validModel, gpuUnits: -1 }).success).toBe(false)
        expect(modelSchema.safeParse({ ...validModel, maxConcurrency: 2.5 }).success).toBe(false)
        expect(modelSchema.safeParse({ ...validModel, absoluteScore: Number.NaN }).success).toBe(false)
        expect(modelSchema.safeParse({ ...validModel, coldStartMs: -10 }).success).toBe(false)
    })
})

describe('tenantSchema', () => {
    const validTenant = {
        displayName: 'tenant-a',
        priority: 'P1',
        qps: 100,
        ttftThresholdMs: 2000,
        queueThreshold: 50,
        ttftScaleDownThresholdMs: 1000,
        queueScaleDownThreshold: 20,
    }

    it('接受合法租户参数', () => {
        expect(tenantSchema.safeParse(validTenant).success).toBe(true)
    })

    it('拒绝非法优先级与非整数 QPS', () => {
        expect(tenantSchema.safeParse({ ...validTenant, priority: 'P9' }).success).toBe(false)
        expect(tenantSchema.safeParse({ ...validTenant, qps: 1.5 }).success).toBe(false)
    })

    it('缩容阈值必须严格小于扩容阈值（与 CRD XValidation 一致）', () => {
        const result = tenantSchema.safeParse({
            ...validTenant,
            ttftScaleDownThresholdMs: 2000,
            queueScaleDownThreshold: 50,
        })
        expect(result.success).toBe(false)
        if (!result.success) {
            const paths = result.error.issues.map((issue) => issue.path.join('.'))
            expect(paths).toEqual(
                expect.arrayContaining(['ttftScaleDownThresholdMs', 'queueScaleDownThreshold']),
            )
        }
    })
})

describe('nodeSchema', () => {
    it('接受合法节点并拒绝零显存/非整数并发', () => {
        expect(nodeSchema.safeParse({ displayName: 'node-1', gpu: 8, maxConcurrency: 4 }).success).toBe(true)
        expect(nodeSchema.safeParse({ displayName: 'node-1', gpu: 0, maxConcurrency: 4 }).success).toBe(false)
        expect(nodeSchema.safeParse({ displayName: 'node-1', gpu: 8, maxConcurrency: 1.5 }).success).toBe(false)
    })
})

describe('orchestratorSchema', () => {
    const validOrchestrator = {
        tenantName: 'tenant-a',
        scaleUpCooldownSeconds: 60,
        scaleDownCooldownSeconds: 120,
        allowScaleToZero: true,
        minReplicas: 1,
        maxReplicas: 0,
        maxScaleUpBatch: 0,
    }

    it('接受合法编排参数（maxReplicas=0 表示无限制）', () => {
        expect(orchestratorSchema.safeParse(validOrchestrator).success).toBe(true)
    })

    it('maxReplicas 非零时最小副本不能超过最大副本', () => {
        const result = orchestratorSchema.safeParse({
            ...validOrchestrator,
            minReplicas: 5,
            maxReplicas: 3,
        })
        expect(result.success).toBe(false)
        if (!result.success) {
            expect(result.error.issues[0].path.join('.')).toBe('minReplicas')
        }
    })
})

describe('policySchema', () => {
    it('tenantModel 必须选择租户与模型', () => {
        const result = policySchema.safeParse({
            kind: 'tenantModel',
            tenantName: '',
            modelName: '',
            nodeName: '',
            effect: 'Allow',
        })
        expect(result.success).toBe(false)
        if (!result.success) {
            const paths = result.error.issues.map((issue) => issue.path.join('.'))
            expect(paths).toEqual(expect.arrayContaining(['tenantName', 'modelName']))
        }
    })

    it('modelNode 必须选择模型与节点，且 effect 只能是 Allow/Deny', () => {
        const result = policySchema.safeParse({
            kind: 'modelNode',
            tenantName: '',
            modelName: 'm',
            nodeName: 'n',
            effect: 'Maybe',
        })
        expect(result.success).toBe(false)
        expect(policySchema.safeParse({
            kind: 'modelNode',
            tenantName: '',
            modelName: 'm',
            nodeName: 'n',
            effect: 'Deny',
        }).success).toBe(true)
    })
})
