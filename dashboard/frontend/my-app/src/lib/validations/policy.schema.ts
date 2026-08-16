import { z } from 'zod'
import type { PreviewConfig } from '@/types/config.types'

export const policySchema = z
    .object({
        kind: z.enum(['tenantModel', 'tenantNode', 'modelNode']),
        tenantName: z.string().trim(),
        modelName: z.string().trim(),
        nodeName: z.string().trim(),
        effect: z.enum(['Allow', 'Deny'], { error: '效果必须是 Allow 或 Deny' }),
    })
    .superRefine((data, context) => {
        if (data.kind === 'tenantModel') {
            if (!data.tenantName) {
                context.addIssue({ code: 'custom', path: ['tenantName'], message: '请选择租户' })
            }
            if (!data.modelName) {
                context.addIssue({ code: 'custom', path: ['modelName'], message: '请选择模型' })
            }
        } else if (data.kind === 'tenantNode') {
            if (!data.tenantName) {
                context.addIssue({ code: 'custom', path: ['tenantName'], message: '请选择租户' })
            }
            if (!data.nodeName) {
                context.addIssue({ code: 'custom', path: ['nodeName'], message: '请选择节点' })
            }
        } else {
            if (!data.modelName) {
                context.addIssue({ code: 'custom', path: ['modelName'], message: '请选择模型' })
            }
            if (!data.nodeName) {
                context.addIssue({ code: 'custom', path: ['nodeName'], message: '请选择节点' })
            }
        }
    })

export type PolicyFormValues = z.infer<typeof policySchema>

export const getPolicyPreview = (data: PolicyFormValues): PreviewConfig => [
    ...(data.tenantName ? [{ key: '租户', value: data.tenantName }] : []),
    ...(data.modelName ? [{ key: '模型', value: data.modelName }] : []),
    ...(data.nodeName ? [{ key: '节点', value: data.nodeName }] : []),
    { key: '效果', value: data.effect },
]