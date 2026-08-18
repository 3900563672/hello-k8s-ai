import { z } from 'zod'
import type { PreviewConfig } from '@/types/config.types'

const nonNegativeInt = (label: string) =>
    z.number({ error: `${label}必须是数字` }).int(`${label}必须是整数`).finite(`${label}必须是有限数值`).nonnegative(`${label}不能为负`)

const positiveInt = (label: string) =>
    z.number({ error: `${label}必须是数字` }).int(`${label}必须是整数`).finite(`${label}必须是有限数值`).positive(`${label}必须大于 0`)

export const orchestratorSchema = z
    .object({
        tenantName: z.string().trim().min(1, '必须选择关联租户'),
        scaleUpCooldownSeconds: nonNegativeInt('扩容冷却时间'),
        scaleDownCooldownSeconds: nonNegativeInt('缩容冷却时间'),
        allowScaleToZero: z.boolean(),
        minReplicas: positiveInt('最小副本数'),
        maxReplicas: nonNegativeInt('最大副本数'),
        maxScaleUpBatch: nonNegativeInt('单次扩容步长'),
    })
    .superRefine((data, context) => {
        // 与 CRD XValidation 保持一致：最小副本数不能超过最大副本数（maxReplicas=0 表示无限制，跳过比较）
        if (data.maxReplicas !== 0 && data.minReplicas > data.maxReplicas) {
            context.addIssue({
                code: 'custom',
                path: ['minReplicas'],
                message: '最小副本数不能大于最大副本数',
            })
        }
    })

export type OrchestratorFormValues = z.infer<typeof orchestratorSchema>

export const getOrchestratorPreview = (data: OrchestratorFormValues): PreviewConfig => [
    { key: '关联租户', value: data.tenantName },
    { key: '扩容冷却', value: data.scaleUpCooldownSeconds, unit: 's' },
    { key: '缩容冷却', value: data.scaleDownCooldownSeconds, unit: 's' },
    { key: '副本范围', value: `${data.minReplicas} - ${data.maxReplicas === 0 ? '∞' : data.maxReplicas}` },
    { key: '扩容步长', value: data.maxScaleUpBatch === 0 ? '默认 10' : `${data.maxScaleUpBatch} 副本/轮` },
    { key: '允许缩到零', value: data.allowScaleToZero ? '是' : '否' },
]
