import type {
    OverlayInstance,
    TrafficPoint,
    TrafficTemplate,
} from '@/types/traffic.types'

export const TRAFFIC_COLORS = [
    '#5B8CFF',
    '#43C6AC',
    '#F6B73C',
    '#B27CFF',
    '#FF7A90',
    '#44B9F1',
]

const EPSILON = 1e-7

export function sanitizeControlPoints(points: TrafficPoint[]): TrafficPoint[] {
    const sorted = points
        .filter((point) => Number.isFinite(point.x) && Number.isFinite(point.y))
        .map((point) => ({ x: Math.max(0, point.x), y: Math.max(0, point.y) }))
        .sort((a, b) => a.x - b.x)

    const result: TrafficPoint[] = []
    for (const point of sorted) {
        const previous = result[result.length - 1]
        if (previous && Math.abs(previous.x - point.x) <= EPSILON) {
            result[result.length - 1] = point
        } else {
            result.push(point)
        }
    }
    return result
}

export function getTemplateDuration(template: Pick<TrafficTemplate, 'controlPoints'>): number {
    const points = sanitizeControlPoints(template.controlPoints)
    return points.at(-1)?.x ?? 0
}

export function getTemplatePeakQps(template: Pick<TrafficTemplate, 'controlPoints'>): number {
    return template.controlPoints.reduce(
        (peak, point) => Number.isFinite(point.y) ? Math.max(peak, point.y) : peak,
        0,
    )
}

/** 在真实秒/QPS 坐标上进行分段线性插值。 */
export function getTemplateValueAtTime(
    template: Pick<TrafficTemplate, 'controlPoints'>,
    elapsedSeconds: number,
): number {
    const points = sanitizeControlPoints(template.controlPoints)
    return interpolateSanitizedPoints(points, elapsedSeconds)
}

function interpolateSanitizedPoints(points: TrafficPoint[], elapsedSeconds: number): number {
    if (points.length === 0 || !Number.isFinite(elapsedSeconds)) return 0

    const first = points[0]
    const last = points[points.length - 1]
    if (elapsedSeconds < first.x - EPSILON || elapsedSeconds > last.x + EPSILON) return 0
    if (Math.abs(elapsedSeconds - first.x) <= EPSILON) return first.y
    if (Math.abs(elapsedSeconds - last.x) <= EPSILON) return last.y

    let low = 0
    let high = points.length - 1
    while (low + 1 < high) {
        const middle = Math.floor((low + high) / 2)
        if (points[middle].x <= elapsedSeconds) low = middle
        else high = middle
    }

    const start = points[low]
    const end = points[high]
    const span = end.x - start.x
    if (span <= EPSILON) return end.y
    const ratio = (elapsedSeconds - start.x) / span
    return Math.max(0, start.y + (end.y - start.y) * ratio)
}

export function getOverlayEndSeconds(
    overlay: Pick<OverlayInstance, 'startOffsetSeconds' | 'templateId'>,
    templates: TrafficTemplate[],
): number {
    const template = templates.find((item) => item.id === overlay.templateId)
    return Math.max(0, overlay.startOffsetSeconds) + (template ? getTemplateDuration(template) : 0)
}

export function getScenarioHorizon(
    templates: TrafficTemplate[],
    overlays: OverlayInstance[],
    minimumHorizonSeconds = 300,
): number {
    const activeEnds = overlays
        .filter((overlay) => overlay.enabled)
        .map((overlay) => getOverlayEndSeconds(overlay, templates))
    const lastEnd = Math.max(0, ...activeEnds)
    const padding = lastEnd > minimumHorizonSeconds
        ? Math.max(15, lastEnd * 0.08)
        : 0
    return Math.max(1, minimumHorizonSeconds, Math.ceil(lastEnd + padding))
}

function niceCeiling(value: number): number {
    if (!Number.isFinite(value) || value <= 1) return 1
    const exponent = Math.floor(Math.log10(value))
    const magnitude = 10 ** exponent
    const fraction = value / magnitude
    const nice = fraction <= 1 ? 1 : fraction <= 2 ? 2 : fraction <= 5 ? 5 : 10
    return nice * magnitude
}

/**
 * 创建长度受控的渲染坐标轴，不限制业务时间范围。坐标轴包含全部模板节点，
 * 保证插值结果准确。
 */
export function buildScenarioTimePoints(
    templates: TrafficTemplate[],
    overlays: OverlayInstance[],
    horizonSeconds: number,
): number[] {
    const horizon = Math.max(1, horizonSeconds)
    const step = niceCeiling(horizon / 720)
    const values = new Set<number>([0, horizon])

    for (let value = 0; value <= horizon; value += step) {
        values.add(Math.min(horizon, value))
    }

    for (const overlay of overlays) {
        if (!overlay.enabled) continue
        const template = templates.find((item) => item.id === overlay.templateId)
        if (!template) continue
        const points = sanitizeControlPoints(template.controlPoints)
        if (points.length === 0) continue

        const offset = Math.max(0, overlay.startOffsetSeconds)
        for (const point of points) {
            const time = offset + point.x
            if (time >= 0 && time <= horizon) values.add(time)
        }

        // 即使曲线起点或终点大于零，也保持边界清晰。
        const firstTime = offset + points[0].x
        const lastTime = offset + points[points.length - 1].x
        const boundaryGap = Math.max(0.001, Math.min(0.05, step / 100))
        if (firstTime > 0) values.add(Math.max(0, firstTime - boundaryGap))
        if (lastTime < horizon) values.add(Math.min(horizon, lastTime + boundaryGap))
    }

    return [...values].sort((a, b) => a - b)
}

export function getTenantSeriesValues(
    tenantId: string,
    timePoints: number[],
    templates: TrafficTemplate[],
    overlays: OverlayInstance[],
): number[] {
    const templateMap = new Map(
        templates.map((template) => [template.id, sanitizeControlPoints(template.controlPoints)]),
    )
    const activeOverlays = overlays.filter(
        (overlay) => overlay.enabled && overlay.tenantId === tenantId,
    )

    return timePoints.map((timeSeconds) => {
        let value = 0
        for (const overlay of activeOverlays) {
            const points = templateMap.get(overlay.templateId)
            if (!points) continue
            value += interpolateSanitizedPoints(
                points,
                timeSeconds - Math.max(0, overlay.startOffsetSeconds),
            )
        }
        return Math.max(0, value)
    })
}

export function getTotalSeriesValues(
    tenantIds: string[],
    timePoints: number[],
    templates: TrafficTemplate[],
    overlays: OverlayInstance[],
): number[] {
    const total = Array.from({ length: timePoints.length }, () => 0)
    for (const tenantId of tenantIds) {
        const values = getTenantSeriesValues(tenantId, timePoints, templates, overlays)
        for (let index = 0; index < values.length; index += 1) {
            total[index] += values[index]
        }
    }
    return total
}

export function formatLogicalTime(seconds: number): string {
    if (!Number.isFinite(seconds)) return '0s'
    const safe = Math.max(0, seconds)
    if (safe < 10 && !Number.isInteger(safe)) return `${Number(safe.toFixed(1))}s`
    const rounded = Math.round(safe)
    const hours = Math.floor(rounded / 3600)
    const minutes = Math.floor((rounded % 3600) / 60)
    const remainingSeconds = rounded % 60
    if (hours > 0) {
        return `${hours}h${minutes > 0 ? ` ${minutes}m` : ''}${remainingSeconds > 0 ? ` ${remainingSeconds}s` : ''}`
    }
    if (minutes > 0) return `${minutes}m${remainingSeconds > 0 ? ` ${remainingSeconds}s` : ''}`
    return `${remainingSeconds}s`
}

export function formatQps(value: number): string {
    const safe = Math.max(0, Number.isFinite(value) ? value : 0)
    if (safe >= 1_000_000_000) return `${Number((safe / 1_000_000_000).toFixed(1))}B`
    if (safe >= 1_000_000) return `${Number((safe / 1_000_000).toFixed(1))}M`
    if (safe >= 1_000) return `${Number((safe / 1_000).toFixed(1))}k`
    return Math.round(safe).toLocaleString('zh-CN')
}
