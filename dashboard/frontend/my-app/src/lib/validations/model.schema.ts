import { z } from 'zod'
import type { PreviewConfig } from '@/types/config.types'

const nonNegativeMetric = (label: string) =>
    z.number({ error: `${label}必须是数字` }).int(`${label}必须是整数`).finite(`${label}必须是有限数值`).nonnegative(`${label}不能为负`)

const positiveMetric = (label: string) =>
    z.number({ error: `${label}必须是数字` }).int(`${label}必须是整数`).finite(`${label}必须是有限数值`).positive(`${label}必须大于 0`)

export const modelSchema = z.object({
    displayName: z.string().trim().min(1, '模型名称不能为空'),
    gpuUnits: positiveMetric('显存'),
    maxConcurrency: positiveMetric('最大并发'),
    absoluteScore: positiveMetric('能力基准分'),
    coldStartMs: nonNegativeMetric('冷启动时间'),
    performance: z.object({
        prefillBaseMs: positiveMetric('Prefill 基础延时'),
        prefillPerTokenUs: positiveMetric('Prefill 每 Token 延时'),
        decodePerTokenMs: positiveMetric('Decode 每 Token 延时'),
    }),
})

export type ModelFormValues = z.infer<typeof modelSchema>

export const getModelPreview = (data: ModelFormValues): PreviewConfig => [
    { key: '显存需求', value: data.gpuUnits, unit: 'G' },
    { key: '最大并发', value: data.maxConcurrency },
    { key: '能力基准分', value: data.absoluteScore },
    { key: '冷启动', value: data.coldStartMs, unit: 'ms' },
    { key: 'Prefill 基础延时', value: data.performance.prefillBaseMs, unit: 'ms' },
    { key: 'Prefill / Token', value: data.performance.prefillPerTokenUs, unit: 'µs' },
    { key: 'Decode / Token', value: data.performance.decodePerTokenMs, unit: 'ms' },
]
