import { create } from 'zustand'
import { devtools } from 'zustand/middleware'
import { useShallow } from 'zustand/react/shallow'
import {
    clampViewport,
    findNearestSnapshot,
    findSnapshotAtOrBefore,
    getTimelineBounds,
    snapshotTime,
    sortSnapshots,
    type TimelineViewport,
} from '@/components/shared/TimeTravelBar/timelineMath'
import type { Snapshot } from '@/types/time.types'

export type TimeMode = 'latest' | 'historical'

export interface ReplayTimeContext {
    effectiveAt: string
    snapshotId: string | null
    mode: TimeMode
    revision: number
}

export interface TimeState {
    timestamp: string
    selectedSnapshotId: string | null
    mode: TimeMode
    revision: number
    snapshots: Snapshot[]
    viewport: TimelineViewport
    setTimestamp: (timestamp: string) => void
    selectSnapshot: (snapshotId: string) => void
    jumpToNearest: (timestamp: string | number) => void
    jumpToTimestamp: (timestamp: string | number) => void
    stepSnapshot: (direction: -1 | 1) => void
    returnToLatest: () => void
    setViewport: (viewport: TimelineViewport) => void
    focusDuration: (durationMs: number | null) => void
    revealSelected: () => void
    resetViewport: () => void
    setSnapshots: (snapshots: Snapshot[]) => void
    setLatestServerTime: (timestamp: string) => void
}

const initialTimestamp = new Date(0).toISOString()
const initialBounds = getTimelineBounds([])

const modeFor = (items: Snapshot[], snapshotId: string | null): TimeMode =>
    items.length === 0 || items.at(-1)?.id === snapshotId ? 'latest' : 'historical'

const selectionPatch = (
    state: Pick<TimeState, 'snapshots' | 'revision'>,
    snapshot: Snapshot,
) => ({
    timestamp: snapshot.timestamp,
    selectedSnapshotId: snapshot.id,
    mode: modeFor(state.snapshots, snapshot.id),
    revision: state.revision + 1,
})

export const useTimeStore = create<TimeState>()(
    devtools(
        (set, get) => ({
            timestamp: initialTimestamp,
            selectedSnapshotId: null,
            mode: 'latest',
            revision: 0,
            snapshots: [],
            viewport: initialBounds,

            setTimestamp: (timestamp) => get().jumpToTimestamp(timestamp),

            selectSnapshot: (snapshotId) => {
                const state = get()
                const snapshot = state.snapshots.find((item) => item.id === snapshotId)
                if (!snapshot) return
                if (snapshot.id === state.selectedSnapshotId) {
                    get().revealSelected()
                    return
                }
                set(selectionPatch(state, snapshot), false, 'time/select-snapshot')
                get().revealSelected()
            },

            jumpToNearest: (timestamp) => {
                const state = get()
                const snapshot = findNearestSnapshot(state.snapshots, timestamp)
                if (!snapshot) return
                set(selectionPatch(state, snapshot), false, 'time/jump-nearest')
                get().revealSelected()
            },

            jumpToTimestamp: (timestamp) => {
                const state = get()
                const snapshot = findSnapshotAtOrBefore(state.snapshots, timestamp)
                if (!snapshot) return
                set(selectionPatch(state, snapshot), false, 'time/jump-at-or-before')
                get().revealSelected()
            },

            stepSnapshot: (direction) => {
                const state = get()
                if (state.snapshots.length === 0) return
                const currentIndex = state.snapshots.findIndex(
                    (snapshot) => snapshot.id === state.selectedSnapshotId,
                )
                const fallback = direction < 0 ? state.snapshots.length : -1
                const nextIndex = Math.max(
                    0,
                    Math.min(
                        state.snapshots.length - 1,
                        (currentIndex >= 0 ? currentIndex : fallback) + direction,
                    ),
                )
                const snapshot = state.snapshots[nextIndex]
                set(selectionPatch(state, snapshot), false, 'time/step-snapshot')
                get().revealSelected()
            },

            returnToLatest: () => {
                const state = get()
                const latest = state.snapshots.at(-1)
                if (!latest) return
                set(selectionPatch(state, latest), false, 'time/return-latest')
                get().revealSelected()
            },

            setViewport: (viewport) => {
                const bounds = getTimelineBounds(get().snapshots)
                set({ viewport: clampViewport(viewport, bounds) }, false, 'time/set-viewport')
            },

            focusDuration: (durationMs) => {
                const state = get()
                const bounds = getTimelineBounds(state.snapshots)
                if (durationMs === null || durationMs >= bounds.end - bounds.start) {
                    set({ viewport: bounds }, false, 'time/focus-all')
                    return
                }
                const selected = state.snapshots.find(
                    (snapshot) => snapshot.id === state.selectedSnapshotId,
                )
                const center = selected ? snapshotTime(selected) : bounds.end
                set(
                    {
                        viewport: clampViewport(
                            { start: center - durationMs / 2, end: center + durationMs / 2 },
                            bounds,
                        ),
                    },
                    false,
                    'time/focus-duration',
                )
            },

            revealSelected: () => {
                const state = get()
                const selected = state.snapshots.find(
                    (snapshot) => snapshot.id === state.selectedSnapshotId,
                )
                if (!selected) return
                const selectedTime = snapshotTime(selected)
                if (selectedTime >= state.viewport.start && selectedTime <= state.viewport.end) return
                const bounds = getTimelineBounds(state.snapshots)
                const span = Math.max(1_000, state.viewport.end - state.viewport.start)
                set(
                    {
                        viewport: clampViewport(
                            { start: selectedTime - span / 2, end: selectedTime + span / 2 },
                            bounds,
                        ),
                    },
                    false,
                    'time/reveal-selected',
                )
            },

            resetViewport: () => {
                set({ viewport: getTimelineBounds(get().snapshots) }, false, 'time/reset-viewport')
            },

            setSnapshots: (nextSnapshots) => {
                const state = get()
                const ordered = sortSnapshots(nextSnapshots)
                const selected = state.mode === 'latest'
                    ? ordered.at(-1) ?? null
                    : ordered.find((snapshot) => snapshot.id === state.selectedSnapshotId)
                        ?? findSnapshotAtOrBefore(ordered, state.timestamp)
                        ?? ordered.at(-1)
                        ?? null
                const bounds = getTimelineBounds(ordered)
                const previousBounds = getTimelineBounds(state.snapshots)
                const followsLatest = state.mode === 'latest'
                    || state.viewport.end >= previousBounds.end - 1
                const previousSpan = Math.max(1_000, state.viewport.end - state.viewport.start)
                const viewport = followsLatest
                    ? clampViewport(
                        { start: bounds.end - previousSpan, end: bounds.end },
                        bounds,
                    )
                    : clampViewport(state.viewport, bounds)
                set(
                    {
                        snapshots: ordered,
                        timestamp: selected?.timestamp ?? state.timestamp,
                        selectedSnapshotId: selected?.id ?? null,
                        mode: modeFor(ordered, selected?.id ?? null),
                        revision: state.revision + 1,
                        viewport,
                    },
                    false,
                    'time/set-server-snapshots',
                )
            },

            setLatestServerTime: (timestamp) => {
                const state = get()
                const parsed = Date.parse(timestamp)
                if (!Number.isFinite(parsed) || parsed <= 0 || state.snapshots.length > 0 || state.mode !== 'latest') return
                set(
                    {
                        timestamp: new Date(parsed).toISOString(),
                        viewport: { start: parsed - 60 * 60 * 1_000, end: parsed },
                        revision: state.revision + 1,
                    },
                    false,
                    'time/set-latest-server-time',
                )
            },
        }),
        { name: 'time-store' },
    ),
)

export const selectTimestamp = (state: TimeState): string => state.timestamp
export const selectSnapshotId = (state: TimeState): string | null => state.selectedSnapshotId
export const selectTimeMode = (state: TimeState): TimeMode => state.mode
export const selectTimeRevision = (state: TimeState): number => state.revision
export const selectSelectedSnapshot = (state: TimeState): Snapshot | null =>
    state.snapshots.find((snapshot) => snapshot.id === state.selectedSnapshotId) ?? null

export const useReplayTimeContext = (): ReplayTimeContext =>
    useTimeStore(
        useShallow((state) => ({
            effectiveAt: state.timestamp,
            snapshotId: state.selectedSnapshotId,
            mode: state.mode,
            revision: state.revision,
        })),
    )

export const getReplayTimeContext = (): ReplayTimeContext => {
    const state = useTimeStore.getState()
    return {
        effectiveAt: state.timestamp,
        snapshotId: state.selectedSnapshotId,
        mode: state.mode,
        revision: state.revision,
    }
}
