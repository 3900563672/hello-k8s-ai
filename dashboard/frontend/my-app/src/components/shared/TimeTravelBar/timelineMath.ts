import type { Snapshot, SnapshotTrigger } from '@/types/time.types'

export interface TimelineBounds {
    start: number
    end: number
}

export interface TimelineViewport extends TimelineBounds {}

export interface TimelineBucket {
    key: string
    timestamp: number
    start: number
    end: number
    count: number
    timeCount: number
    eventCount: number
    timeScore: number
    eventScore: number
    peakWeight: number
    snapshotIds: string[]
    representativeId: string
}

export interface TimelineGranularity {
    bucketMs: number
    label: string
}

const SECOND = 1_000
const MINUTE = 60 * SECOND
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

const NICE_BUCKETS = [
    SECOND,
    2 * SECOND,
    5 * SECOND,
    10 * SECOND,
    15 * SECOND,
    30 * SECOND,
    MINUTE,
    2 * MINUTE,
    5 * MINUTE,
    10 * MINUTE,
    15 * MINUTE,
    30 * MINUTE,
    HOUR,
    2 * HOUR,
    4 * HOUR,
    6 * HOUR,
    12 * HOUR,
    DAY,
    2 * DAY,
    7 * DAY,
    14 * DAY,
    30 * DAY,
] as const

const validTime = (value: string | number): number => {
    const parsed = typeof value === 'number' ? value : Date.parse(value)
    return Number.isFinite(parsed) ? parsed : 0
}

export const snapshotTime = (snapshot: Pick<Snapshot, 'timestamp'>): number =>
    validTime(snapshot.timestamp)

export const sortSnapshots = (snapshots: Snapshot[]): Snapshot[] =>
    [...snapshots].sort((left, right) => snapshotTime(left) - snapshotTime(right))

export const getTimelineBounds = (snapshots: Snapshot[]): TimelineBounds => {
    if (snapshots.length === 0) {
        const now = Date.now()
        return { start: now - HOUR, end: now }
    }

    let start = Number.POSITIVE_INFINITY
    let end = Number.NEGATIVE_INFINITY
    for (const snapshot of snapshots) {
        const time = snapshotTime(snapshot)
        start = Math.min(start, time)
        end = Math.max(end, time)
    }

    if (start === end) {
        return { start: start - 30 * SECOND, end: end + 30 * SECOND }
    }
    return { start, end }
}

export const clampViewport = (
    viewport: TimelineViewport,
    bounds: TimelineBounds,
    minimumSpan = SECOND,
): TimelineViewport => {
    const fullSpan = Math.max(minimumSpan, bounds.end - bounds.start)
    const requestedSpan = Math.max(
        minimumSpan,
        Math.min(fullSpan, viewport.end - viewport.start),
    )
    let start = Number.isFinite(viewport.start) ? viewport.start : bounds.start
    let end = start + requestedSpan

    if (start < bounds.start) {
        start = bounds.start
        end = start + requestedSpan
    }
    if (end > bounds.end) {
        end = bounds.end
        start = end - requestedSpan
    }

    return {
        start: Math.max(bounds.start, start),
        end: Math.min(bounds.end, end),
    }
}

export const viewportEquals = (
    left: TimelineViewport,
    right: TimelineViewport,
    tolerance = 2,
): boolean =>
    Math.abs(left.start - right.start) <= tolerance &&
    Math.abs(left.end - right.end) <= tolerance

const bucketLabel = (bucketMs: number): string => {
    if (bucketMs < MINUTE) return Math.round(bucketMs / SECOND) + ' 秒'
    if (bucketMs < HOUR) return Math.round(bucketMs / MINUTE) + ' 分钟'
    if (bucketMs < DAY) return Math.round(bucketMs / HOUR) + ' 小时'
    return Math.round(bucketMs / DAY) + ' 天'
}

export const chooseGranularity = (
    viewportSpan: number,
    pixelWidth: number,
): TimelineGranularity => {
    const targetBars = Math.max(24, Math.min(180, Math.floor(pixelWidth / 7)))
    const rawBucket = Math.max(SECOND, viewportSpan / targetBars)
    const bucketMs =
        NICE_BUCKETS.find((candidate) => candidate >= rawBucket) ??
        NICE_BUCKETS[NICE_BUCKETS.length - 1]
    return { bucketMs, label: bucketLabel(bucketMs) }
}

const scoreFor = (count: number, peakWeight: number): number => {
    if (count <= 0) return 0
    return Number((Math.log2(count + 1) * 2.4 + peakWeight * 0.55).toFixed(3))
}

export const aggregateSnapshots = (
    snapshots: Snapshot[],
    bounds: TimelineBounds,
    bucketMs: number,
): TimelineBucket[] => {
    const buckets = new Map<
        number,
        {
            timeSnapshots: Snapshot[]
            eventSnapshots: Snapshot[]
            peakWeight: number
        }
    >()

    for (const snapshot of snapshots) {
        const time = snapshotTime(snapshot)
        if (time < bounds.start || time > bounds.end) continue
        const start =
            bounds.start + Math.floor((time - bounds.start) / bucketMs) * bucketMs
        const current = buckets.get(start) ?? {
            timeSnapshots: [],
            eventSnapshots: [],
            peakWeight: 0,
        }
        const target =
            snapshot.trigger === 'event' ? current.eventSnapshots : current.timeSnapshots
        target.push(snapshot)
        current.peakWeight = Math.max(current.peakWeight, snapshot.weight)
        buckets.set(start, current)
    }

    return [...buckets.entries()]
        .sort(([left], [right]) => left - right)
        .map(([start, group]) => {
            const combined = [...group.timeSnapshots, ...group.eventSnapshots].sort(
                (left, right) => snapshotTime(left) - snapshotTime(right),
            )
            const timePeak = group.timeSnapshots.reduce(
                (peak, snapshot) => Math.max(peak, snapshot.weight),
                0,
            )
            const eventPeak = group.eventSnapshots.reduce(
                (peak, snapshot) => Math.max(peak, snapshot.weight),
                0,
            )
            const representative =
                group.eventSnapshots.at(-1) ?? group.timeSnapshots.at(-1) ?? combined[0]

            return {
                key: String(start),
                timestamp: start + bucketMs / 2,
                start,
                end: start + bucketMs,
                count: combined.length,
                timeCount: group.timeSnapshots.length,
                eventCount: group.eventSnapshots.length,
                timeScore: scoreFor(group.timeSnapshots.length, timePeak),
                eventScore: scoreFor(group.eventSnapshots.length, eventPeak),
                peakWeight: group.peakWeight,
                snapshotIds: combined.map((snapshot) => snapshot.id),
                representativeId: representative.id,
            }
        })
}

export const findNearestSnapshot = (
    snapshots: Snapshot[],
    target: string | number,
): Snapshot | null => {
    if (snapshots.length === 0) return null
    const targetTime = validTime(target)
    let nearest = snapshots[0]
    let distance = Math.abs(snapshotTime(nearest) - targetTime)

    for (let index = 1; index < snapshots.length; index += 1) {
        const candidate = snapshots[index]
        const candidateDistance = Math.abs(snapshotTime(candidate) - targetTime)
        if (candidateDistance < distance) {
            nearest = candidate
            distance = candidateDistance
        }
    }
    return nearest
}

/**
 * 回放语义使用“目标时刻之前的最后一个切面”，避免读取到未来状态。
 */
export const findSnapshotAtOrBefore = (
    snapshots: Snapshot[],
    target: string | number,
): Snapshot | null => {
    if (snapshots.length === 0) return null
    const targetTime = validTime(target)
    let low = 0
    let high = snapshots.length - 1
    let answer = -1

    while (low <= high) {
        const middle = Math.floor((low + high) / 2)
        if (snapshotTime(snapshots[middle]) <= targetTime) {
            answer = middle
            low = middle + 1
        } else {
            high = middle - 1
        }
    }

    return answer >= 0 ? snapshots[answer] : snapshots[0]
}

export const countSnapshotsInViewport = (
    snapshots: Snapshot[],
    viewport: TimelineViewport,
): number =>
    snapshots.reduce((count, snapshot) => {
        const time = snapshotTime(snapshot)
        return count + (time >= viewport.start && time <= viewport.end ? 1 : 0)
    }, 0)

export const countByTrigger = (
    snapshots: Snapshot[],
    trigger: SnapshotTrigger,
): number =>
    snapshots.reduce(
        (count, snapshot) => count + (snapshot.trigger === trigger ? 1 : 0),
        0,
    )

const twoDigits = (value: number): string => String(value).padStart(2, '0')
const threeDigits = (value: number): string => String(value).padStart(3, '0')

export const formatUtc = (
    value: string | number,
    includeMilliseconds = false,
): string => {
    const date = new Date(validTime(value))
    const datePart =
        date.getUTCFullYear() +
        '-' +
        twoDigits(date.getUTCMonth() + 1) +
        '-' +
        twoDigits(date.getUTCDate())
    const timePart =
        twoDigits(date.getUTCHours()) +
        ':' +
        twoDigits(date.getUTCMinutes()) +
        ':' +
        twoDigits(date.getUTCSeconds())
    return (
        datePart +
        ' ' +
        timePart +
        (includeMilliseconds ? '.' + threeDigits(date.getUTCMilliseconds()) : '')
    )
}

export const formatAxisUtc = (value: number, visibleSpan: number): string => {
    const date = new Date(value)
    if (visibleSpan <= 2 * MINUTE) {
        return (
            twoDigits(date.getUTCHours()) +
            ':' +
            twoDigits(date.getUTCMinutes()) +
            ':' +
            twoDigits(date.getUTCSeconds())
        )
    }
    if (visibleSpan <= 2 * DAY) {
        return (
            twoDigits(date.getUTCMonth() + 1) +
            '-' +
            twoDigits(date.getUTCDate()) +
            ' ' +
            twoDigits(date.getUTCHours()) +
            ':' +
            twoDigits(date.getUTCMinutes())
        )
    }
    if (visibleSpan <= 120 * DAY) {
        return (
            twoDigits(date.getUTCMonth() + 1) +
            '-' +
            twoDigits(date.getUTCDate())
        )
    }
    return (
        date.getUTCFullYear() +
        '-' +
        twoDigits(date.getUTCMonth() + 1)
    )
}

export const formatDuration = (durationMs: number): string => {
    const seconds = Math.max(0, Math.round(durationMs / SECOND))
    if (seconds < 60) return seconds + ' 秒'
    const minutes = Math.round(seconds / 60)
    if (minutes < 60) return minutes + ' 分钟'
    const hours = Math.round(minutes / 60)
    if (hours < 48) return hours + ' 小时'
    const days = Math.round(hours / 24)
    if (days < 60) return days + ' 天'
    const months = Number((days / 30.4375).toFixed(1))
    return months + ' 个月'
}

export const toUtcDateTimeInput = (value: string | number): string => {
    const date = new Date(validTime(value))
    return (
        date.getUTCFullYear() +
        '-' +
        twoDigits(date.getUTCMonth() + 1) +
        '-' +
        twoDigits(date.getUTCDate()) +
        'T' +
        twoDigits(date.getUTCHours()) +
        ':' +
        twoDigits(date.getUTCMinutes()) +
        ':' +
        twoDigits(date.getUTCSeconds()) +
        '.' +
        threeDigits(date.getUTCMilliseconds())
    )
}

export const parseUtcDateTimeInput = (value: string): number | null => {
    if (!value) return null
    const normalized = value.endsWith('Z') ? value : value + 'Z'
    const parsed = Date.parse(normalized)
    return Number.isFinite(parsed) ? parsed : null
}

export const escapeHtml = (value: string): string =>
    value
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#039;')

export const triggerLabel = (trigger: SnapshotTrigger): string =>
    trigger === 'time' ? '时间驱动' : '事件驱动'
