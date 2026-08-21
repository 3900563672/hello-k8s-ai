import { describe, expect, it } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useFullscreenTimeline } from '@/hooks/useFullscreenTimeline'

describe('useFullscreenTimeline', () => {
    it('默认关闭，show/hide/setOpen 控制状态', () => {
        const { result } = renderHook(() => useFullscreenTimeline())
        expect(result.current.open).toBe(false)
        act(() => result.current.show())
        expect(result.current.open).toBe(true)
        act(() => result.current.hide())
        expect(result.current.open).toBe(false)
        act(() => result.current.setOpen(true))
        expect(result.current.open).toBe(true)
    })

    it('initialOpen 初始为开', () => {
        const { result } = renderHook(() => useFullscreenTimeline(true))
        expect(result.current.open).toBe(true)
    })
})