import { describe, expect, it } from 'vitest'
import { formatUtcTimestamp } from '@/lib/formatters/timeFormatter'

describe('formatUtcTimestamp', () => {
    it('把 ISO 字符串格式化为 UTC 可读时间', () => {
        expect(formatUtcTimestamp('2026-08-12T12:00:00.000Z')).toBe('2026-08-12 12:00:00')
    })

    it('includeMs 时保留毫秒', () => {
        expect(formatUtcTimestamp('2026-08-12T12:00:00.123Z', true)).toBe('2026-08-12 12:00:00.123')
    })

    it('接受毫秒时间戳数字', () => {
        expect(formatUtcTimestamp(Date.parse('2026-08-12T12:00:00.000Z'))).toBe('2026-08-12 12:00:00')
    })

    it('非法输入返回 Invalid timestamp', () => {
        expect(formatUtcTimestamp('not-a-date')).toBe('Invalid timestamp')
        expect(formatUtcTimestamp(Number.NaN)).toBe('Invalid timestamp')
    })
})
