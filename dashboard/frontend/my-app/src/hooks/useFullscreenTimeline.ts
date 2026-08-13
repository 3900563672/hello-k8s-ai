import { useCallback, useState } from 'react'

export function useFullscreenTimeline(initialOpen = false) {
    const [open, setOpen] = useState(initialOpen)
    const show = useCallback(() => setOpen(true), [])
    const hide = useCallback(() => setOpen(false), [])

    return { open, setOpen, show, hide }
}
