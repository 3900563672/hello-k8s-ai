import { useEffect, useRef } from 'react'
import { API_BASE_URL } from '@/api/client'
import { fetchReplayTimeline } from '@/api/endpoints/controlPlaneApi'
import { useControlPlaneStore } from '@/stores/controlPlaneSlice'
import { useTimeStore } from '@/stores/timeSlice'

const SAFE_RESYNC_MS = 30_000

export function useBackendSync() {
    const refreshCluster = useControlPlaneStore((state) => state.refreshCluster)
    const setSnapshots = useTimeStore((state) => state.setSnapshots)
    const setLatestServerTime = useTimeStore((state) => state.setLatestServerTime)
    const syncing = useRef(false)
    const debounceTimer = useRef<number | null>(null)

    useEffect(() => {
        let active = true
        const sync = async () => {
            if (syncing.current || !active) return
            syncing.current = true
            try {
                const [, timeline] = await Promise.allSettled([
                    refreshCluster(),
                    fetchReplayTimeline(),
                ])
                if (active && timeline.status === 'fulfilled' && timeline.value.length > 0) {
                    setSnapshots(timeline.value)
                } else if (active) {
                    setLatestServerTime(useControlPlaneStore.getState().cluster.serverTime)
                }
            } finally {
                syncing.current = false
            }
        }
        const scheduleSync = () => {
            if (debounceTimer.current !== null) window.clearTimeout(debounceTimer.current)
            debounceTimer.current = window.setTimeout(() => void sync(), 350)
        }

        void sync()
        const interval = window.setInterval(() => void sync(), SAFE_RESYNC_MS)
        const stream = new EventSource(`${API_BASE_URL}/stream?topics=resources,events,clock`)
        stream.addEventListener('resource.changed', scheduleSync)
        stream.addEventListener('resync-required', scheduleSync)
        const handleVisibility = () => {
            if (document.visibilityState === 'visible') void sync()
        }
        document.addEventListener('visibilitychange', handleVisibility)

        return () => {
            active = false
            window.clearInterval(interval)
            if (debounceTimer.current !== null) window.clearTimeout(debounceTimer.current)
            stream.close()
            document.removeEventListener('visibilitychange', handleVisibility)
        }
    }, [refreshCluster, setLatestServerTime, setSnapshots])
}
