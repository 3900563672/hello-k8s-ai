import { z } from 'zod'
import type { PreviewConfig } from '@/types/config.types'

export const nodeSchema = z.object({
    displayName: z.string().trim().min(1, '节点名称不能为空'),
    gpu: z.number({ error: '总显存必须是数字' }).finite().positive('总显存必须大于 0'),
    maxConcurrency: z.number({ error: '并发数必须是数字' }).int('并发数必须是整数').positive('并发数必须大于 0'),
})

export type NodeFormValues = z.infer<typeof nodeSchema>

export const getNodePreview = (data: NodeFormValues): PreviewConfig => [
    { key: '总显存', value: data.gpu, unit: 'G' },
    { key: '最大并发', value: data.maxConcurrency },
]
