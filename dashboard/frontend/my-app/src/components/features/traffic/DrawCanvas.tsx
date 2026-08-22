import {
    useCallback,
    useEffect,
    useMemo,
    useRef,
    useState,
    type ChangeEvent,
    type KeyboardEvent as ReactKeyboardEvent,
    type PointerEvent as ReactPointerEvent,
    type WheelEvent as ReactWheelEvent,
} from 'react'
import {
    Activity,
    Hand,
    Maximize2,
    PenLine,
    Redo2,
    RotateCcw,
    Save,
    Trash2,
    Undo2,
    X,
    ZoomIn,
    ZoomOut,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

interface Point {
    x: number
    y: number
}

interface Camera {
    x: number
    y: number
    zoom: number
}

interface CanvasSize {
    width: number
    height: number
}

interface CursorInfo {
    screenX: number
    screenY: number
    world: Point
}

type CanvasTool = 'draw' | 'pan'

type DragState =
    | { kind: 'draw'; pointerId: number }
    | {
    kind: 'pan'
    pointerId: number
    startX: number
    startY: number
    camera: Camera
}
    | null

interface DrawCanvasProps {
    onSave: (name: string, points: Point[]) => void
    onCancel: () => void
}

const BASE_X_SCALE = 8 // px / second
const BASE_Y_SCALE = 0.72 // px / QPS
// 仅用于保护渲染，不是业务限制；范围覆盖亚秒级到多日、百万 QPS 场景。
const MIN_ZOOM = 0.0001
const MAX_ZOOM = 12

const palette = {
    surface: '#07090F',
    surfaceRaised: '#0C1018',
    gridMinor: 'rgba(148, 163, 184, 0.055)',
    gridMajor: 'rgba(148, 163, 184, 0.12)',
    ruler: 'rgba(148, 163, 184, 0.22)',
    label: '#718096',
    labelStrong: '#A6B1C2',
    line: '#5EA2FF',
    lineBright: '#8CC8FF',
    accent: '#2F81F7',
}

function getPlotBounds(size: CanvasSize) {
    return {
        left: 72,
        top: 22,
        right: Math.max(73, size.width - 24),
        bottom: Math.max(23, size.height - 52),
    }
}

function clamp(value: number, min: number, max: number) {
    return Math.min(max, Math.max(min, value))
}

function niceStep(rawStep: number) {
    if (!Number.isFinite(rawStep) || rawStep <= 0) return 1
    const exponent = Math.floor(Math.log10(rawStep))
    const fraction = rawStep / 10 ** exponent
    const niceFraction = fraction <= 1 ? 1 : fraction <= 2 ? 2 : fraction <= 5 ? 5 : 10
    return niceFraction * 10 ** exponent
}

function formatTime(value: number) {
    if (value >= 3600) return `${Number((value / 3600).toFixed(1))}h`
    if (value >= 60) return `${Number((value / 60).toFixed(1))}m`
    if (value >= 10) return `${Math.round(value)}s`
    return `${Number(value.toFixed(1))}s`
}

function formatQps(value: number) {
    if (value >= 1_000_000) return `${Number((value / 1_000_000).toFixed(1))}M`
    if (value >= 1_000) return `${Number((value / 1_000).toFixed(1))}k`
    return `${Math.round(value)}`
}

function flattenStrokes(strokes: Point[][]) {
    const result: Point[] = []
    for (const stroke of strokes) {
        for (const point of stroke) {
            const last = result[result.length - 1]
            if (!last || Math.abs(last.x - point.x) > 0.0001 || Math.abs(last.y - point.y) > 0.0001) {
                result.push(point)
            }
        }
    }
    return result
}

function makeSmoothPath(points: Point[]) {
    const path = new Path2D()
    if (points.length === 0) return path

    path.moveTo(points[0].x, points[0].y)
    if (points.length === 2) {
        path.lineTo(points[1].x, points[1].y)
        return path
    }

    for (let index = 1; index < points.length - 1; index += 1) {
        const current = points[index]
        const next = points[index + 1]
        const midpoint = {
            x: (current.x + next.x) / 2,
            y: (current.y + next.y) / 2,
        }
        path.quadraticCurveTo(current.x, current.y, midpoint.x, midpoint.y)
    }
    const last = points[points.length - 1]
    path.lineTo(last.x, last.y)
    return path
}

function distanceToSegmentSquared(point: Point, start: Point, end: Point) {
    const pointX = point.x * BASE_X_SCALE
    const pointY = point.y * BASE_Y_SCALE
    const startX = start.x * BASE_X_SCALE
    const startY = start.y * BASE_Y_SCALE
    const endX = end.x * BASE_X_SCALE
    const endY = end.y * BASE_Y_SCALE
    const deltaX = endX - startX
    const deltaY = endY - startY
    if (Math.abs(deltaX) + Math.abs(deltaY) < 0.000001) {
        return (pointX - startX) ** 2 + (pointY - startY) ** 2
    }
    const ratio = clamp(
        ((pointX - startX) * deltaX + (pointY - startY) * deltaY)
        / (deltaX ** 2 + deltaY ** 2),
        0,
        1,
    )
    const nearestX = startX + deltaX * ratio
    const nearestY = startY + deltaY * ratio
    return (pointX - nearestX) ** 2 + (pointY - nearestY) ** 2
}

function simplifyPoints(points: Point[], tolerance = 0.9) {
    if (points.length <= 2) return points
    const keep = new Uint8Array(points.length)
    keep[0] = 1
    keep[points.length - 1] = 1
    const ranges: Array<[number, number]> = [[0, points.length - 1]]
    const toleranceSquared = tolerance ** 2

    while (ranges.length > 0) {
        const [startIndex, endIndex] = ranges.pop()!
        let farthestIndex = -1
        let farthestDistance = 0
        for (let index = startIndex + 1; index < endIndex; index += 1) {
            const distance = distanceToSegmentSquared(
                points[index],
                points[startIndex],
                points[endIndex],
            )
            if (distance > farthestDistance) {
                farthestDistance = distance
                farthestIndex = index
            }
        }
        if (farthestIndex > startIndex && farthestDistance > toleranceSquared) {
            keep[farthestIndex] = 1
            ranges.push([startIndex, farthestIndex], [farthestIndex, endIndex])
        }
    }

    return points.filter((_, index) => keep[index] === 1)
}

function prepareControlPoints(points: Point[]) {
    if (points.length < 2) return []

    // 保留实际坐标，只移除无效、倒序和时间坐标重复的采样点。
    const monotonic: Point[] = []
    for (const point of points) {
        if (!Number.isFinite(point.x) || !Number.isFinite(point.y)) continue
        const safePoint = { x: Math.max(0, point.x), y: Math.max(0, point.y) }
        const last = monotonic[monotonic.length - 1]
        if (!last) {
            monotonic.push(safePoint)
        } else if (safePoint.x > last.x + 0.0001) {
            monotonic.push(safePoint)
        } else if (Math.abs(safePoint.x - last.x) <= 0.0001) {
            monotonic[monotonic.length - 1] = safePoint
        }
    }

    if (monotonic.length < 2) return []
    const simplified = simplifyPoints(monotonic)
    return simplified.map((point) => ({
        x: Number(point.x.toFixed(3)),
        y: Number(point.y.toFixed(2)),
    }))
}

function drawScene(
    ctx: CanvasRenderingContext2D,
    size: CanvasSize,
    camera: Camera,
    strokes: Point[][],
    cursor: CursorInfo | null
) {
    const { width, height } = size
    const plot = getPlotBounds(size)
    const plotWidth = plot.right - plot.left
    const plotHeight = plot.bottom - plot.top
    if (plotWidth <= 1 || plotHeight <= 1) return

    ctx.clearRect(0, 0, width, height)

    const background = ctx.createLinearGradient(0, 0, 0, height)
    background.addColorStop(0, '#090C13')
    background.addColorStop(1, palette.surface)
    ctx.fillStyle = background
    ctx.fillRect(0, 0, width, height)

    const xScale = BASE_X_SCALE * camera.zoom
    const yScale = BASE_Y_SCALE * camera.zoom
    const xMax = camera.x + plotWidth / xScale
    const yMax = camera.y + plotHeight / yScale
    const xStep = niceStep(96 / xScale)
    const yStep = niceStep(82 / yScale)
    const xMinorStep = xStep / 5
    const yMinorStep = yStep / 5

    const worldToScreen = (point: Point) => ({
        x: plot.left + (point.x - camera.x) * xScale,
        y: plot.bottom - (point.y - camera.y) * yScale,
    })

    ctx.save()
    ctx.beginPath()
    ctx.rect(plot.left, plot.top, plotWidth, plotHeight)
    ctx.clip()

    // 次级网格随相机移动，平移和缩放时保持连续的正坐标平面。
    ctx.lineWidth = 1
    ctx.strokeStyle = palette.gridMinor
    ctx.beginPath()
    for (let x = Math.ceil(camera.x / xMinorStep) * xMinorStep; x <= xMax; x += xMinorStep) {
        const screenX = plot.left + (x - camera.x) * xScale
        ctx.moveTo(screenX, plot.top)
        ctx.lineTo(screenX, plot.bottom)
    }
    for (let y = Math.ceil(camera.y / yMinorStep) * yMinorStep; y <= yMax; y += yMinorStep) {
        const screenY = plot.bottom - (y - camera.y) * yScale
        ctx.moveTo(plot.left, screenY)
        ctx.lineTo(plot.right, screenY)
    }
    ctx.stroke()

    ctx.strokeStyle = palette.gridMajor
    ctx.beginPath()
    for (let x = Math.ceil(camera.x / xStep) * xStep; x <= xMax; x += xStep) {
        const screenX = plot.left + (x - camera.x) * xScale
        ctx.moveTo(screenX, plot.top)
        ctx.lineTo(screenX, plot.bottom)
    }
    for (let y = Math.ceil(camera.y / yStep) * yStep; y <= yMax; y += yStep) {
        const screenY = plot.bottom - (y - camera.y) * yScale
        ctx.moveTo(plot.left, screenY)
        ctx.lineTo(plot.right, screenY)
    }
    ctx.stroke()

    const worldPoints = flattenStrokes(strokes)
    if (worldPoints.length > 0) {
        const screenPoints = worldPoints.map(worldToScreen)
        const curve = makeSmoothPath(screenPoints)

        if (screenPoints.length > 1) {
            const area = new Path2D(curve)
            const last = screenPoints[screenPoints.length - 1]
            const first = screenPoints[0]
            area.lineTo(last.x, plot.bottom)
            area.lineTo(first.x, plot.bottom)
            area.closePath()
            const fill = ctx.createLinearGradient(0, plot.top, 0, plot.bottom)
            fill.addColorStop(0, 'rgba(47, 129, 247, 0.22)')
            fill.addColorStop(0.7, 'rgba(47, 129, 247, 0.06)')
            fill.addColorStop(1, 'rgba(47, 129, 247, 0)')
            ctx.fillStyle = fill
            ctx.fill(area)
        }

        ctx.save()
        ctx.strokeStyle = 'rgba(47, 129, 247, 0.38)'
        ctx.lineWidth = 8
        ctx.shadowBlur = 18
        ctx.shadowColor = 'rgba(47, 129, 247, 0.55)'
        ctx.stroke(curve)
        ctx.restore()

        const lineGradient = ctx.createLinearGradient(plot.left, 0, plot.right, 0)
        lineGradient.addColorStop(0, palette.line)
        lineGradient.addColorStop(1, palette.lineBright)
        ctx.strokeStyle = lineGradient
        ctx.lineWidth = 2.4
        ctx.lineCap = 'round'
        ctx.lineJoin = 'round'
        ctx.stroke(curve)

        const first = screenPoints[0]
        const last = screenPoints[screenPoints.length - 1]
        for (const [point, color] of [[first, palette.line], [last, '#D4ECFF']] as const) {
            ctx.beginPath()
            ctx.arc(point.x, point.y, 4.5, 0, Math.PI * 2)
            ctx.fillStyle = palette.surfaceRaised
            ctx.fill()
            ctx.lineWidth = 2
            ctx.strokeStyle = color
            ctx.stroke()
        }
    }

    if (cursor) {
        ctx.save()
        ctx.setLineDash([3, 5])
        ctx.strokeStyle = 'rgba(140, 200, 255, 0.24)'
        ctx.lineWidth = 1
        ctx.beginPath()
        ctx.moveTo(cursor.screenX, plot.top)
        ctx.lineTo(cursor.screenX, plot.bottom)
        ctx.moveTo(plot.left, cursor.screenY)
        ctx.lineTo(plot.right, cursor.screenY)
        ctx.stroke()
        ctx.restore()
    }
    ctx.restore()

    // 标尺背景固定在视口边缘，刻度和值随相机移动。
    const leftRuler = ctx.createLinearGradient(0, 0, plot.left, 0)
    leftRuler.addColorStop(0, '#080B11')
    leftRuler.addColorStop(1, 'rgba(8, 11, 17, 0.94)')
    ctx.fillStyle = leftRuler
    ctx.fillRect(0, plot.top, plot.left, plotHeight)
    const bottomRuler = ctx.createLinearGradient(0, plot.bottom, 0, height)
    bottomRuler.addColorStop(0, 'rgba(8, 11, 17, 0.94)')
    bottomRuler.addColorStop(1, '#07090F')
    ctx.fillStyle = bottomRuler
    ctx.fillRect(plot.left, plot.bottom, plotWidth, height - plot.bottom)

    ctx.strokeStyle = palette.ruler
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(plot.left, plot.top)
    ctx.lineTo(plot.left, plot.bottom)
    ctx.lineTo(plot.right, plot.bottom)
    ctx.stroke()

    ctx.font = '500 11px Geist, ui-sans-serif, system-ui, sans-serif'
    ctx.fillStyle = palette.label
    ctx.textAlign = 'center'
    ctx.textBaseline = 'top'
    for (let x = Math.ceil(camera.x / xStep) * xStep; x <= xMax; x += xStep) {
        const screenX = plot.left + (x - camera.x) * xScale
        if (screenX < plot.left + 12 || screenX > plot.right - 12) continue
        ctx.beginPath()
        ctx.moveTo(screenX, plot.bottom)
        ctx.lineTo(screenX, plot.bottom + 5)
        ctx.stroke()
        ctx.fillText(formatTime(x), screenX, plot.bottom + 9)
    }

    ctx.textAlign = 'right'
    ctx.textBaseline = 'middle'
    for (let y = Math.ceil(camera.y / yStep) * yStep; y <= yMax; y += yStep) {
        const screenY = plot.bottom - (y - camera.y) * yScale
        if (screenY < plot.top + 10 || screenY > plot.bottom - 8) continue
        ctx.beginPath()
        ctx.moveTo(plot.left - 5, screenY)
        ctx.lineTo(plot.left, screenY)
        ctx.stroke()
        ctx.fillText(formatQps(y), plot.left - 10, screenY)
    }

    ctx.fillStyle = palette.labelStrong
    ctx.font = '600 10px Geist, ui-sans-serif, system-ui, sans-serif'
    ctx.textAlign = 'left'
    ctx.textBaseline = 'top'
    ctx.fillText('QPS', 18, 14)
    ctx.textAlign = 'right'
    ctx.textBaseline = 'bottom'
    ctx.fillText('时间', width - 24, height - 10)

    if (camera.x < 0.0001 && camera.y < 0.0001) {
        ctx.beginPath()
        ctx.arc(plot.left, plot.bottom, 3.5, 0, Math.PI * 2)
        ctx.fillStyle = palette.accent
        ctx.fill()
        ctx.fillStyle = palette.labelStrong
        ctx.font = '600 10px Geist, ui-sans-serif, system-ui, sans-serif'
        ctx.textAlign = 'right'
        ctx.textBaseline = 'top'
        ctx.fillText('0', plot.left - 9, plot.bottom + 8)
    }

    if (cursor) {
        const label = `${formatTime(cursor.world.x)}  ·  ${formatQps(cursor.world.y)} QPS`
        ctx.font = '600 11px Geist, ui-sans-serif, system-ui, sans-serif'
        const labelWidth = ctx.measureText(label).width + 20
        const labelX = clamp(cursor.screenX + 12, plot.left + 8, plot.right - labelWidth - 8)
        const labelY = clamp(cursor.screenY - 38, plot.top + 8, plot.bottom - 32)
        ctx.fillStyle = 'rgba(13, 18, 28, 0.96)'
        ctx.beginPath()
        ctx.roundRect(labelX, labelY, labelWidth, 28, 7)
        ctx.fill()
        ctx.strokeStyle = 'rgba(94, 162, 255, 0.28)'
        ctx.stroke()
        ctx.fillStyle = '#C9D8EA'
        ctx.textAlign = 'left'
        ctx.textBaseline = 'middle'
        ctx.fillText(label, labelX + 10, labelY + 14)
    }
}

export function DrawCanvas({ onSave, onCancel }: DrawCanvasProps) {
    const canvasRef = useRef<HTMLCanvasElement>(null)
    const dragRef = useRef<DragState>(null)
    const spacePressedRef = useRef(false)
    const [size, setSize] = useState<CanvasSize>({ width: 0, height: 0 })
    const [camera, setCamera] = useState<Camera>({ x: 0, y: 0, zoom: 1 })
    const [strokes, setStrokes] = useState<Point[][]>([])
    const [redoStack, setRedoStack] = useState<Point[][]>([])
    const [tool, setTool] = useState<CanvasTool>('draw')
    const [cursor, setCursor] = useState<CursorInfo | null>(null)
    const [isInteracting, setIsInteracting] = useState(false)
    const [spacePressed, setSpacePressed] = useState(false)
    const [notice, setNotice] = useState<string | null>(null)
    const [templateName, setTemplateName] = useState('')

    const points = useMemo(() => flattenStrokes(strokes), [strokes])
    const plot = useMemo(() => getPlotBounds(size), [size])
    const visibleTime = Math.max(0, (plot.right - plot.left) / (BASE_X_SCALE * camera.zoom))
    const visibleQps = Math.max(0, (plot.bottom - plot.top) / (BASE_Y_SCALE * camera.zoom))

    useEffect(() => {
        const canvas = canvasRef.current
        const host = canvas?.parentElement
        if (!canvas || !host) return

        const resize = () => {
            const rect = host.getBoundingClientRect()
            setSize({ width: Math.max(1, rect.width), height: Math.max(1, rect.height) })
        }
        resize()
        const observer = new ResizeObserver(resize)
        observer.observe(host)
        return () => observer.disconnect()
    }, [])

    useEffect(() => {
        const canvas = canvasRef.current
        if (!canvas || size.width <= 0 || size.height <= 0) return
        const dpr = Math.min(2, window.devicePixelRatio || 1)
        canvas.width = Math.round(size.width * dpr)
        canvas.height = Math.round(size.height * dpr)
        canvas.style.width = `${size.width}px`
        canvas.style.height = `${size.height}px`
        const ctx = canvas.getContext('2d')
        if (!ctx) return
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
        drawScene(ctx, size, camera, strokes, cursor)
    }, [camera, cursor, size, strokes])

    const resetView = useCallback(() => {
        setCamera({ x: 0, y: 0, zoom: 1 })
    }, [])

    const zoomAt = useCallback((factor: number, screenPoint?: { x: number; y: number }) => {
        setCamera((current) => {
            const bounds = getPlotBounds(size)
            const anchor = screenPoint ?? {
                x: (bounds.left + bounds.right) / 2,
                y: (bounds.top + bounds.bottom) / 2,
            }
            const nextZoom = clamp(current.zoom * factor, MIN_ZOOM, MAX_ZOOM)
            const worldX = current.x + (anchor.x - bounds.left) / (BASE_X_SCALE * current.zoom)
            const worldY = current.y + (bounds.bottom - anchor.y) / (BASE_Y_SCALE * current.zoom)
            return {
                x: Math.max(0, worldX - (anchor.x - bounds.left) / (BASE_X_SCALE * nextZoom)),
                y: Math.max(0, worldY - (bounds.bottom - anchor.y) / (BASE_Y_SCALE * nextZoom)),
                zoom: nextZoom,
            }
        })
    }, [size])

    const fitCurve = useCallback(() => {
        if (points.length < 2) {
            resetView()
            return
        }
        const bounds = getPlotBounds(size)
        const minX = Math.min(...points.map((point) => point.x))
        const maxX = Math.max(...points.map((point) => point.x))
        const maxY = Math.max(1, ...points.map((point) => point.y))
        const widthZoom = (bounds.right - bounds.left) / (Math.max(1, maxX - minX) * BASE_X_SCALE * 1.18)
        const heightZoom = (bounds.bottom - bounds.top) / (maxY * BASE_Y_SCALE * 1.18)
        const zoom = clamp(Math.min(widthZoom, heightZoom), MIN_ZOOM, MAX_ZOOM)
        setCamera({
            x: Math.max(0, minX - (bounds.right - bounds.left) * 0.05 / (BASE_X_SCALE * zoom)),
            y: 0,
            zoom,
        })
    }, [points, resetView, size])

    const undo = useCallback(() => {
        setStrokes((current) => {
            if (current.length === 0) return current
            const removed = current[current.length - 1]
            setRedoStack((redo) => [removed, ...redo])
            return current.slice(0, -1)
        })
    }, [])

    const redo = useCallback(() => {
        setRedoStack((current) => {
            if (current.length === 0) return current
            const [restored, ...rest] = current
            setStrokes((existing) => [...existing, restored])
            return rest
        })
    }, [])

    const clearCurve = useCallback(() => {
        setStrokes((current) => {
            if (current.length === 0) return current
            // 清空不可重做：redo 栈是单笔粒度，整批清空混入会让重做把整个
            // strokes 数组当作单笔恢复，导致笔数/采样点错乱（#185）。
            setRedoStack([])
            return []
        })
    }, [])

    useEffect(() => {
        const isTypingTarget = (target: EventTarget | null) => {
            const element = target as HTMLElement | null
            return element?.tagName === 'INPUT' || element?.tagName === 'TEXTAREA' || element?.isContentEditable
        }

        const handleKeyDown = (event: KeyboardEvent) => {
            if (isTypingTarget(event.target)) return
            if (event.code === 'Space') {
                event.preventDefault()
                spacePressedRef.current = true
                setSpacePressed(true)
            } else if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'z') {
                event.preventDefault()
                if (event.shiftKey) redo()
                else undo()
            } else if (event.key.toLowerCase() === 'p') {
                setTool('draw')
            } else if (event.key.toLowerCase() === 'h') {
                setTool('pan')
            } else if (event.key === '0') {
                resetView()
            }
        }
        const handleKeyUp = (event: KeyboardEvent) => {
            if (event.code === 'Space') {
                spacePressedRef.current = false
                setSpacePressed(false)
            }
        }
        window.addEventListener('keydown', handleKeyDown)
        window.addEventListener('keyup', handleKeyUp)
        return () => {
            window.removeEventListener('keydown', handleKeyDown)
            window.removeEventListener('keyup', handleKeyUp)
        }
    }, [redo, resetView, undo])

    const getLocalPoint = (event: ReactPointerEvent<HTMLCanvasElement>) => {
        const rect = event.currentTarget.getBoundingClientRect()
        return { x: event.clientX - rect.left, y: event.clientY - rect.top }
    }

    const screenToWorld = useCallback((screen: Point) => ({
        x: Math.max(0, camera.x + (screen.x - plot.left) / (BASE_X_SCALE * camera.zoom)),
        y: Math.max(0, camera.y + (plot.bottom - screen.y) / (BASE_Y_SCALE * camera.zoom)),
    }), [camera, plot])

    const isInsidePlot = useCallback((point: Point) => (
        point.x >= plot.left && point.x <= plot.right && point.y >= plot.top && point.y <= plot.bottom
    ), [plot])

    const handlePointerDown = (event: ReactPointerEvent<HTMLCanvasElement>) => {
        const local = getLocalPoint(event)
        const shouldPan = tool === 'pan' || spacePressedRef.current || event.button === 1

        if (shouldPan) {
            event.preventDefault()
            event.currentTarget.setPointerCapture(event.pointerId)
            dragRef.current = {
                kind: 'pan',
                pointerId: event.pointerId,
                startX: local.x,
                startY: local.y,
                camera,
            }
            setIsInteracting(true)
            return
        }

        if (event.button !== 0 || !isInsidePlot(local)) return
        event.currentTarget.setPointerCapture(event.pointerId)
        const world = screenToWorld(local)
        setRedoStack([])
        setStrokes((current) => {
            const lastStroke = current[current.length - 1]
            const lastPoint = lastStroke?.[lastStroke.length - 1]
            if (!lastPoint) return [...current, [world]]
            // 每次落笔都从现有尾点继续；多次手势最终保存为一条曲线。
            return [...current, [lastPoint, world.x >= lastPoint.x ? world : lastPoint]]
        })
        dragRef.current = { kind: 'draw', pointerId: event.pointerId }
        setIsInteracting(true)
    }

    const handlePointerMove = (event: ReactPointerEvent<HTMLCanvasElement>) => {
        const local = getLocalPoint(event)
        const clampedScreen = {
            x: clamp(local.x, plot.left, plot.right),
            y: clamp(local.y, plot.top, plot.bottom),
        }
        const world = screenToWorld(clampedScreen)
        setCursor(isInsidePlot(local) ? { screenX: local.x, screenY: local.y, world } : null)

        const drag = dragRef.current
        if (!drag || drag.pointerId !== event.pointerId) return

        if (drag.kind === 'pan') {
            const deltaX = local.x - drag.startX
            const deltaY = local.y - drag.startY
            setCamera({
                x: Math.max(0, drag.camera.x - deltaX / (BASE_X_SCALE * drag.camera.zoom)),
                y: Math.max(0, drag.camera.y + deltaY / (BASE_Y_SCALE * drag.camera.zoom)),
                zoom: drag.camera.zoom,
            })
            return
        }

        setStrokes((current) => {
            if (current.length === 0) return current
            const stroke = current[current.length - 1]
            const last = stroke[stroke.length - 1]
            if (!last || world.x < last.x) return current
            const dx = (world.x - last.x) * BASE_X_SCALE * camera.zoom
            const dy = (world.y - last.y) * BASE_Y_SCALE * camera.zoom
            if (Math.hypot(dx, dy) < 2.2) return current
            const updated = [...stroke, world]
            return [...current.slice(0, -1), updated]
        })
    }

    const finishPointerInteraction = (event: ReactPointerEvent<HTMLCanvasElement>) => {
        const drag = dragRef.current
        if (!drag || drag.pointerId !== event.pointerId) return
        if (drag.kind === 'draw') {
            setStrokes((current) => {
                const last = current[current.length - 1]
                return last && flattenStrokes([last]).length < 2 ? current.slice(0, -1) : current
            })
        }
        if (event.currentTarget.hasPointerCapture(event.pointerId)) {
            event.currentTarget.releasePointerCapture(event.pointerId)
        }
        dragRef.current = null
        setIsInteracting(false)
    }

    const handleWheel = (event: ReactWheelEvent<HTMLCanvasElement>) => {
        event.preventDefault()
        const rect = event.currentTarget.getBoundingClientRect()
        const anchor = { x: event.clientX - rect.left, y: event.clientY - rect.top }
        if (!isInsidePlot(anchor)) return
        zoomAt(Math.exp(-event.deltaY * 0.0014), anchor)
    }

    const showNotice = (message: string) => {
        setNotice(message)
        window.setTimeout(() => setNotice(null), 2600)
    }

    const handleSave = () => {
        const name = templateName.trim()
        if (!name) {
            showNotice('请先为模板填写名称')
            return
        }
        const prepared = prepareControlPoints(points)
        if (prepared.length < 2) {
            showNotice('请从左向右绘制一段更长的曲线')
            return
        }
        onSave(name, prepared)
    }

    const handleCancel = () => onCancel()

    const activePan = tool === 'pan' || spacePressed
    const cursorClass = activePan
        ? isInteracting ? 'cursor-grabbing' : 'cursor-grab'
        : 'cursor-crosshair'

    return (
        <div className="flex h-full min-h-0 flex-col bg-[#07090F] text-[#E8EEF7]">
            <header className="flex min-h-[60px] shrink-0 items-center justify-between gap-4 border-b border-white/[0.07] bg-[#090C12]/95 px-5">
                <div className="flex min-w-0 items-center gap-3">
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-[#2F81F7]/30 bg-[#2F81F7]/10 shadow-[0_0_28px_rgba(47,129,247,0.12)]">
                        <Activity className="h-[18px] w-[18px] text-[#67A7FF]" />
                    </div>
                    <div className="min-w-0">
                        <div className="flex items-center gap-2">
                            <h2 className="truncate text-sm font-semibold tracking-[-0.01em] text-[#F3F7FC]">曲线工作台</h2>
                            <span className="rounded-full border border-[#2F81F7]/20 bg-[#2F81F7]/10 px-2 py-0.5 text-[13px] font-semibold uppercase tracking-[0.16em] text-[#75B1FF]">QPS</span>
                        </div>
                        <p className="mt-0.5 truncate text-[14px] text-[#667085]">真实秒 / QPS 坐标 · 多笔自动续接 · 无业务上限</p>
                    </div>
                </div>

                <div className="flex items-center gap-2">
                    <Input
                        value={templateName}
                        onChange={(event: ChangeEvent<HTMLInputElement>) => setTemplateName(event.target.value)}
                        onKeyDown={(event: ReactKeyboardEvent<HTMLInputElement>) => {
                            if (event.key === 'Enter') handleSave()
                        }}
                        aria-label="模板名称"
                        className="h-9 w-[220px] border-white/[0.08] bg-white/[0.035] text-xs text-[#DDE7F3] shadow-none placeholder:text-[#556070] focus-visible:border-[#2F81F7]/60 focus-visible:ring-[#2F81F7]/15"
                        placeholder="输入模板名称（不自动填充）"
                    />
                    <Button
                        variant="ghost"
                        size="sm"
                        onClick={handleCancel}
                        className="h-9 text-[#8B96A8] hover:bg-white/[0.05] hover:text-white"
                    >
                        <X className="mr-1.5 h-4 w-4" />
                        取消
                    </Button>
                    <Button
                        size="sm"
                        onClick={handleSave}
                        className="h-9 bg-[#2F81F7] px-4 text-white shadow-[0_8px_24px_rgba(47,129,247,0.2)] hover:bg-[#4A91F8]"
                    >
                        <Save className="mr-1.5 h-4 w-4" />
                        保存模板
                    </Button>
                </div>
            </header>

            <div className="relative min-h-0 flex-1 overflow-hidden">
                <canvas
                    ref={canvasRef}
                    className={cn('absolute inset-0 block touch-none select-none', cursorClass)}
                    onPointerDown={handlePointerDown}
                    onPointerMove={handlePointerMove}
                    onPointerUp={finishPointerInteraction}
                    onPointerCancel={finishPointerInteraction}
                    onPointerLeave={() => {
                        if (!dragRef.current) setCursor(null)
                    }}
                    onWheel={handleWheel}
                    onContextMenu={(event) => event.preventDefault()}
                    aria-label="QPS 曲线绘制画布"
                />

                <div className="pointer-events-none absolute left-1/2 top-4 z-20 -translate-x-1/2">
                    {notice && (
                        <div className="rounded-lg border border-amber-400/20 bg-[#17140D]/95 px-3.5 py-2 text-xs text-amber-200 shadow-2xl backdrop-blur-xl">
                            {notice}
                        </div>
                    )}
                </div>

                <div className="absolute left-[88px] top-4 z-10 flex items-center gap-1 rounded-xl border border-white/[0.08] bg-[#0D121B]/90 p-1.5 shadow-[0_12px_40px_rgba(0,0,0,0.28)] backdrop-blur-xl">
                    <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setTool('draw')}
                        title="画笔 (P)"
                        className={cn('h-8 w-8 rounded-lg text-[#7D899B] hover:bg-white/[0.06] hover:text-white', tool === 'draw' && 'bg-[#2F81F7]/15 text-[#70AEFF]')}
                    >
                        <PenLine className="h-4 w-4" />
                    </Button>
                    <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setTool('pan')}
                        title="平移 (按住空格可临时使用)"
                        className={cn('h-8 w-8 rounded-lg text-[#7D899B] hover:bg-white/[0.06] hover:text-white', tool === 'pan' && 'bg-[#2F81F7]/15 text-[#70AEFF]')}
                    >
                        <Hand className="h-4 w-4" />
                    </Button>
                    <div className="mx-1 h-5 w-px bg-white/[0.08]" />
                    <Button variant="ghost" size="icon" onClick={undo} disabled={strokes.length === 0} title="撤销 (Ctrl/⌘ Z)" className="h-8 w-8 rounded-lg text-[#7D899B] hover:bg-white/[0.06] hover:text-white disabled:opacity-25">
                        <Undo2 className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" onClick={redo} disabled={redoStack.length === 0} title="重做 (Ctrl/⌘ Shift Z)" className="h-8 w-8 rounded-lg text-[#7D899B] hover:bg-white/[0.06] hover:text-white disabled:opacity-25">
                        <Redo2 className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" onClick={clearCurve} disabled={strokes.length === 0} title="清空曲线" className="h-8 w-8 rounded-lg text-[#7D899B] hover:bg-red-500/10 hover:text-red-300 disabled:opacity-25">
                        <Trash2 className="h-4 w-4" />
                    </Button>
                </div>

                <div className="absolute right-4 top-4 z-10 flex items-center gap-1 rounded-xl border border-white/[0.08] bg-[#0D121B]/90 p-1.5 shadow-[0_12px_40px_rgba(0,0,0,0.28)] backdrop-blur-xl">
                    <Button variant="ghost" size="icon" onClick={() => zoomAt(1.2)} title="放大" className="h-8 w-8 rounded-lg text-[#7D899B] hover:bg-white/[0.06] hover:text-white">
                        <ZoomIn className="h-4 w-4" />
                    </Button>
                    <span className="w-12 text-center font-mono text-[12px] text-[#8B96A8]">{Math.round(camera.zoom * 100)}%</span>
                    <Button variant="ghost" size="icon" onClick={() => zoomAt(1 / 1.2)} title="缩小" className="h-8 w-8 rounded-lg text-[#7D899B] hover:bg-white/[0.06] hover:text-white">
                        <ZoomOut className="h-4 w-4" />
                    </Button>
                    <div className="mx-1 h-5 w-px bg-white/[0.08]" />
                    <Button variant="ghost" size="icon" onClick={fitCurve} title="适配曲线" className="h-8 w-8 rounded-lg text-[#7D899B] hover:bg-white/[0.06] hover:text-white">
                        <Maximize2 className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" onClick={resetView} title="回到原点 (0)" className="h-8 w-8 rounded-lg text-[#7D899B] hover:bg-white/[0.06] hover:text-white">
                        <RotateCcw className="h-4 w-4" />
                    </Button>
                </div>

                {points.length === 0 && (
                    <div className="pointer-events-none absolute inset-0 flex items-center justify-center px-8">
                        <div className="mb-6 max-w-[420px] rounded-2xl border border-white/[0.07] bg-[#0C111A]/76 px-8 py-7 text-center shadow-[0_24px_80px_rgba(0,0,0,0.24)] backdrop-blur-md">
                            <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-2xl border border-[#2F81F7]/25 bg-[#2F81F7]/10 text-[#73B1FF]">
                                <PenLine className="h-5 w-5" />
                            </div>
                            <h3 className="text-sm font-semibold text-[#E8EEF7]">从左向右绘制你的流量曲线</h3>
                            <p className="mt-2 text-xs leading-5 text-[#697588]">松开后可以继续下一笔，系统会自动从上一笔末端无缝续接。保存时保留这里看到的真实秒与 QPS，不做归一化。</p>
                        </div>
                    </div>
                )}

                <div className="pointer-events-none absolute bottom-3 left-[84px] right-4 z-10 flex items-end justify-between gap-3">
                    <div className="rounded-lg border border-white/[0.06] bg-[#0B0F17]/78 px-3 py-2 text-[12px] text-[#657184] backdrop-blur-md">
                        <span className="text-[#93A0B2]">可视范围</span>
                        <span className="mx-2 text-white/15">/</span>
                        {formatTime(camera.x)} – {formatTime(camera.x + visibleTime)}
                        <span className="mx-2 text-white/15">·</span>
                        {formatQps(camera.y)} – {formatQps(camera.y + visibleQps)} QPS
                    </div>
                    <div className="flex items-center gap-3 rounded-lg border border-white/[0.06] bg-[#0B0F17]/78 px-3 py-2 text-[12px] text-[#657184] backdrop-blur-md">
                        <span><span className="text-[#93A0B2]">{strokes.length}</span> 笔</span>
                        <span><span className="text-[#93A0B2]">{points.length}</span> 采样点</span>
                        <span className="hidden xl:inline">真实坐标 · 仅正时间 / 正 QPS</span>
                    </div>
                </div>
            </div>
        </div>
    )
}
