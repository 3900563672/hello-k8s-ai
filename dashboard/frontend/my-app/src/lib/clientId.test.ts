import { afterEach, describe, expect, it, vi } from 'vitest'
import { createClientId } from '@/lib/clientId'

describe('createClientId', () => {
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('优先使用 crypto.randomUUID', () => {
        vi.stubGlobal('crypto', {
            randomUUID: () => '00000000-0000-4000-8000-000000000001',
            getRandomValues: (bytes: Uint8Array) => bytes,
        })
        expect(createClientId('traffic')).toBe('traffic-00000000-0000-4000-8000-000000000001')
    })

    it('无 randomUUID 时退化为 getRandomValues 十六进制', () => {
        const bytes = new Uint8Array([
            0xde, 0xad, 0xbe, 0xef, 0x00, 0x11, 0x22, 0x33,
            0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb,
        ])
        vi.stubGlobal('crypto', { getRandomValues: (buffer: Uint8Array) => {
            buffer.set(bytes)
            return buffer
        } })
        expect(createClientId('simulation-rate')).toBe(
            'simulation-rate-deadbeef00112233445566778899aabb',
        )
    })

    it('无 crypto 时使用时间戳 + 自增序列且不重复', () => {
        vi.stubGlobal('crypto', undefined)
        const first = createClientId('template')
        const second = createClientId('template')
        expect(first).toMatch(/^template-/)
        expect(first).not.toBe(second)
    })
})
