import { z } from 'zod'
import type { PreviewConfig } from '@/types/config.types'

const nonNegativeMetric = (label: string) =>
    z.number({ error: `${label}必须是数字` }).finite(`${label}必须是有限数值`).nonnegative(`${label}不能为负`)

export const tenantSchema = z
    .object({
        displayName: z.string().trim().min(1, '租户名称不能为空'),
        priority: z.enum(['P1', 'P2', 'P3', 'P4', 'P5']),
        qps: nonNegativeMetric('基准 QPS'),
        ttftThresholdMs: nonNegativeMetric('TTFT 扩容阈值'),
        queueThreshold: nonNegativeMetric('队列扩容阈值'),
        ttftScaleDownThresholdMs: nonNegativeMetric('TTFT 缩容阈值'),
        queueScaleDownThreshold: nonNegativeMetric('队列缩容阈值'),
    })
    .superRefine((data, context) => {
        if (data.ttftScaleDownThresholdMs > data.ttftThresholdMs) {
            context.addIssue({
                code: 'custom',
                path: ['ttftScaleDownThresholdMs'],
                message: '缩容阈值不能高于扩容阈值',
            })
        }
        if (data.queueScaleDownThreshold > data.queueThreshold) {
            context.addIssue({
                code: 'custom',
                path: ['queueScaleDownThreshold'],
                message: '缩容阈值不能高于扩容阈值',
            })
        }
    })

export type TenantFormValues = z.infer<typeof tenantSchema>

export const getTenantPreview = (data: TenantFormValues): PreviewConfig => [
    { key: '优先级', value: data.priority },
    { key: '基准流量', value: data.qps, unit: 'QPS' },
    { key: 'TTFT 扩容阈值', value: data.ttftThresholdMs, unit: 'ms' },
    { key: '队列扩容阈值', value: data.queueThreshold },
    { key: 'TTFT 缩容阈值', value: data.ttftScaleDownThresholdMs, unit: 'ms' },
    { key: '队列缩容阈值', value: data.queueScaleDownThreshold },
]
