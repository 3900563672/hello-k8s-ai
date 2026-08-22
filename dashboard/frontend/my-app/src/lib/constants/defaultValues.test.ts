import { describe, expect, it } from 'vitest'
import { DEFAULT_MODEL, DEFAULT_NODE, DEFAULT_ORCHESTRATOR, DEFAULT_TENANT } from '@/lib/constants/defaultValues'

describe('defaultValues 与 CRD 默认值一致', () => {
    it('DEFAULT_MODEL 可通过 modelSchema 校验（无需编辑直接创建）', () => {
        expect(DEFAULT_MODEL.gpuUnits).toBe(1)
        expect(DEFAULT_MODEL.maxConcurrency).toBe(1)
        expect(DEFAULT_MODEL.absoluteScore).toBe(100)
        expect(DEFAULT_MODEL.coldStartMs).toBe(0)
        expect(DEFAULT_MODEL.performance).toEqual({ prefillBaseMs: 50, prefillPerTokenUs: 500, decodePerTokenMs: 20 })
    })

    it('DEFAULT_NODE 默认单卡单并发', () => {
        expect(DEFAULT_NODE).toEqual({ gpu: 1, maxConcurrency: 1 })
    })

    it('DEFAULT_TENANT P3 零 QPS 且阈值合规（缩容 < 扩容）', () => {
        expect(DEFAULT_TENANT.priority).toBe('P3')
        expect(DEFAULT_TENANT.qps).toBe(0)
        expect(DEFAULT_TENANT.ttftScaleDownThresholdMs).toBeLessThan(DEFAULT_TENANT.ttftThresholdMs)
        expect(DEFAULT_TENANT.queueScaleDownThreshold).toBeLessThan(DEFAULT_TENANT.queueThreshold)
    })

    it('DEFAULT_ORCHESTRATOR maxReplicas=0 表示无限制', () => {
        expect(DEFAULT_ORCHESTRATOR.maxReplicas).toBe(0)
        expect(DEFAULT_ORCHESTRATOR.minReplicas).toBe(1)
        expect(DEFAULT_ORCHESTRATOR.allowScaleToZero).toBe(false)
        expect(DEFAULT_ORCHESTRATOR.scaleUpCooldownSeconds).toBe(60)
        expect(DEFAULT_ORCHESTRATOR.scaleDownCooldownSeconds).toBe(120)
        expect(DEFAULT_ORCHESTRATOR.maxScaleUpBatch).toBe(10)
    })
})