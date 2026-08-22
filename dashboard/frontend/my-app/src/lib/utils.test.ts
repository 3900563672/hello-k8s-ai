import { describe, expect, it } from 'vitest'
import { cn } from '@/lib/utils'

describe('cn', () => {
    it('合并类名并去重 tailwind 冲突', () => {
        expect(cn('a', 'b')).toBe('a b')
        expect(cn('p-2', 'p-4')).toBe('p-4')
        expect(cn(null, undefined, 'c')).toBe('c')
    })
})