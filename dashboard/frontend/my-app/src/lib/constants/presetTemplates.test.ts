import { describe, expect, it } from 'vitest'
import {
    PRESET_MODEL_TEMPLATES,
    PRESET_NODE_TEMPLATES,
    PRESET_ORCHESTRATOR_TEMPLATES,
    PRESET_TENANT_TEMPLATES,
    PRESET_TRAFFIC_TEMPLATES,
} from '@/lib/constants/presetTemplates'
import { modelSchema } from '@/lib/validations/model.schema'
import { nodeSchema } from '@/lib/validations/node.schema'
import { orchestratorSchema } from '@/lib/validations/orchestrator.schema'
import { tenantSchema } from '@/lib/validations/tenant.schema'
import { getTemplateDuration } from '@/components/features/traffic/trafficMath'

describe('presetTemplates', () => {
    it('四类配置模板的 id 全局唯一且名称不重复', () => {
        const all = [
            ...PRESET_MODEL_TEMPLATES,
            ...PRESET_NODE_TEMPLATES,
            ...PRESET_TENANT_TEMPLATES,
            ...PRESET_ORCHESTRATOR_TEMPLATES,
        ]
        const ids = all.map((template) => template.id)
        const names = all.map((template) => template.name)
        expect(new Set(ids).size).toBe(ids.length)
        expect(new Set(names).size).toBe(names.length)
        expect(all.length).toBeGreaterThanOrEqual(10)
    })

    it('所有配置模板标记为预置且数据通过对应 zod schema', () => {
        for (const template of PRESET_MODEL_TEMPLATES) {
            expect(template.preset).toBe(true)
            expect(modelSchema.safeParse(template.data).success).toBe(true)
        }
        for (const template of PRESET_NODE_TEMPLATES) {
            expect(nodeSchema.safeParse(template.data).success).toBe(true)
        }
        for (const template of PRESET_ORCHESTRATOR_TEMPLATES) {
            expect(orchestratorSchema.safeParse(template.data).success).toBe(true)
        }
        for (const template of PRESET_TENANT_TEMPLATES) {
            expect(tenantSchema.safeParse(template.data).success).toBe(true)
        }
    })

    it('流量模板至少包含一个，且时长大于 0', () => {
        expect(PRESET_TRAFFIC_TEMPLATES.length).toBeGreaterThan(0)
        for (const template of PRESET_TRAFFIC_TEMPLATES) {
            expect(getTemplateDuration(template)).toBeGreaterThan(0)
            expect(template.controlPoints.length).toBeGreaterThanOrEqual(2)
        }
    })
})