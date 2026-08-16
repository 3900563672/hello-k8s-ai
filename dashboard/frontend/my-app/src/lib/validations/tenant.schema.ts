import { z } from 'zod'
import type { PreviewConfig } from '@/types/config.types'

const nonNegativeMetric = (label: string) =>
    z.number({ error: `${label}必须是数字` }).int(`${label}必须是整数`).finite(`${label}必须是有限数值`).nonnegative(`${label}不能为负`)

const positiveMetric = (label: string) =>
    z.number({ error: `${label}必须是数字` }).int(`${label}必须是整数`).finite(`${label}必须是有限数值`).positive(`${label}必须大于 0`)

export const tenantSchema = z
    .object({
        displayName: z.string().trim().min(1, '租户名称不能为空'),
        priority: z.enum(['P1', 'P2', 'P3', 'P4', 'P5']),
        qps: nonNegativeMetric('基准 QPS'),
        ttftThresholdMs: positiveMetric('TTFT 扩容阈值'),
        queueThreshold: positiveMetric('队列扩容阈值'),
        ttftScaleDownThresholdMs: positiveMetric('TTFT 缩容阈值'),
        queueScaleDownThreshold: positiveMetric('队列缩容阈值'),
    })
    .superRefine((data, context) => {
        // 与 CRD XValidation 保持一致：缩容阈值必须严格小于扩容阈值
        if (data.ttftScaleDownThresholdMs >= data.ttftThresholdMs) {
            context.addIssue({
                code: 'custom',
                path: ['ttftScaleDownThresholdMs'],
                message: '缩容阈值必须小于扩容阈值',
            })
        }
        if (data.queueScaleDownThreshold >= data.queueThreshold) {
            context.addIssue({
                code: 'custom',
                path: ['queueScaleDownThreshold'],
                message: '缩容阈值必须小于扩容阈值',
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
