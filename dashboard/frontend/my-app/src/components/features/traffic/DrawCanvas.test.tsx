import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { DrawCanvas } from './DrawCanvas'

// --- canvas 2D 上下文 mock（jsdom 无实现；drawScene 依赖的方法全部桩掉） ---
const setTransform = vi.fn()

const mockCtx = new Proxy({} as Record<string, unknown>, {
    get(_target, prop) {
        if (prop === 'setTransform') return setTransform
        if (prop === 'createLinearGradient' || prop === 'createRadialGradient') {
            return () => ({ addColorStop: vi.fn() })
        }
        if (prop === 'measureText') return () => ({ width: 10 })
        return () => undefined
    },
    set() {
        return true
    },
})

// drawScene 使用 Path2D（jsdom 缺失）
class Path2DStub {
    moveTo() {}
    lineTo() {}
    quadraticCurveTo() {}
    closePath() {}
    rect() {}
}

// 800x400 的宿主矩形：plot = { left:72, top:22, right:776, bottom:348 }
const RECT = {
    x: 0,
    y: 0,
    width: 800,
    height: 400,
    left: 0,
    top: 0,
    right: 800,
    bottom: 400,
    toJSON: () => ({}),
} as DOMRect

function renderCanvas(onSave = vi.fn(), onCancel = vi.fn()) {
    const utils = render(<DrawCanvas onSave={onSave} onCancel={onCancel} />)
    const canvas = utils.getByLabelText('QPS 曲线绘制画布')
    return { ...utils, canvas, onSave, onCancel }
}

/** 画一笔：down → 逐个 move → up（坐标均为画布局部坐标） */
function drawStroke(canvas: HTMLElement, points: Array<[number, number]>, pointerId = 1) {
    const [firstX, firstY] = points[0]
    fireEvent.pointerDown(canvas, { pointerId, button: 0, clientX: firstX, clientY: firstY })
    for (const [x, y] of points.slice(1)) {
        fireEvent.pointerMove(canvas, { pointerId, clientX: x, clientY: y })
    }
    const [lastX, lastY] = points[points.length - 1]
    fireEvent.pointerUp(canvas, { pointerId, clientX: lastX, clientY: lastY })
}

beforeEach(() => {
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
        mockCtx as unknown as CanvasRenderingContext2D,
    )
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(RECT)
    ;(globalThis as Record<string, unknown>).Path2D = Path2DStub
})

afterEach(() => {
    vi.restoreAllMocks()
})

describe('DrawCanvas 渲染与初始状态', () => {
    it('渲染画布/工具栏/输入框/状态栏，默认画笔工具 100% 视图', () => {
        const { canvas, container } = renderCanvas()
        expect(canvas).toBeInTheDocument()
        expect(canvas.className).toContain('cursor-crosshair')
        expect(screen.getByTitle('画笔 (P)')).toBeInTheDocument()
        expect(screen.getByTitle('平移 (按住空格可临时使用)')).toBeInTheDocument()
        expect(screen.getByTitle('撤销 (Ctrl/⌘ Z)')).toBeDisabled()
        expect(screen.getByTitle('重做 (Ctrl/⌘ Shift Z)')).toBeDisabled()
        expect(screen.getByTitle('清空曲线')).toBeDisabled()
        expect(screen.getByText('100%')).toBeInTheDocument()
        expect(screen.getByText('从左向右绘制你的流量曲线')).toBeInTheDocument()
        expect(container.textContent).toContain('0 笔')
        expect(container.textContent).toContain('0 采样点')
        expect(screen.getByLabelText('模板名称')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: '取消' })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /保存模板/ })).toBeInTheDocument()
    })

    it('挂载后执行 drawScene（2D ctx 的 setTransform 被调用）', async () => {
        renderCanvas()
        await waitFor(() => expect(setTransform).toHaveBeenCalled())
    })
})

describe('DrawCanvas 绘制交互', () => {
    it('从左向右绘制生成采样点，空态消失，撤销可用', () => {
        const { canvas, container } = renderCanvas()
        drawStroke(canvas, [
            [100, 300],
            [200, 300],
            [300, 300],
        ])
        expect(container.textContent).toContain('1 笔')
        expect(container.textContent).toContain('3 采样点')
        expect(screen.queryByText('从左向右绘制你的流量曲线')).not.toBeInTheDocument()
        expect(screen.getByTitle('撤销 (Ctrl/⌘ Z)')).toBeEnabled()
    })

    it('原地单击（无移动）的短笔画被丢弃', () => {
        const { canvas, container } = renderCanvas()
        fireEvent.pointerDown(canvas, { pointerId: 1, button: 0, clientX: 100, clientY: 300 })
        fireEvent.pointerUp(canvas, { pointerId: 1, clientX: 100, clientY: 300 })
        expect(container.textContent).toContain('0 笔')
        expect(container.textContent).toContain('0 采样点')
        expect(screen.getByText('从左向右绘制你的流量曲线')).toBeInTheDocument()
    })

    it('反向移动（x 后退）不追加采样点', () => {
        const { canvas, container } = renderCanvas()
        fireEvent.pointerDown(canvas, { pointerId: 1, button: 0, clientX: 100, clientY: 300 })
        fireEvent.pointerMove(canvas, { pointerId: 1, clientX: 300, clientY: 300 })
        fireEvent.pointerMove(canvas, { pointerId: 1, clientX: 200, clientY: 300 })
        fireEvent.pointerUp(canvas, { pointerId: 1, clientX: 200, clientY: 300 })
        expect(container.textContent).toContain('1 笔')
        expect(container.textContent).toContain('2 采样点')
    })

    it('第二笔自动从上一笔尾点续接', () => {
        const { canvas, container } = renderCanvas()
        drawStroke(canvas, [
            [100, 300],
            [200, 300],
        ])
        fireEvent.pointerDown(canvas, { pointerId: 2, button: 0, clientX: 250, clientY: 300 })
        fireEvent.pointerUp(canvas, { pointerId: 2, clientX: 250, clientY: 300 })
        expect(container.textContent).toContain('2 笔')
        expect(container.textContent).toContain('3 采样点')
    })

    it('plot 外的落笔被忽略', () => {
        const { canvas, container } = renderCanvas()
        drawStroke(canvas, [
            [30, 300],
            [100, 300],
        ])
        expect(container.textContent).toContain('0 笔')
    })

    it('绘制时重复执行 drawScene 不抛错', async () => {
        const { canvas } = renderCanvas()
        const callsBefore = setTransform.mock.calls.length
        drawStroke(canvas, [
            [100, 300],
            [200, 300],
            [300, 300],
        ])
        await waitFor(() => expect(setTransform.mock.calls.length).toBeGreaterThan(callsBefore))
    })
})

describe('DrawCanvas 平移与缩放', () => {
    it('平移工具下拖动画布移动相机（可视范围变化）', () => {
        const { canvas, container } = renderCanvas()
        fireEvent.click(screen.getByTitle('平移 (按住空格可临时使用)'))
        expect(canvas.className).toContain('cursor-grab')
        fireEvent.pointerDown(canvas, { pointerId: 1, button: 0, clientX: 200, clientY: 200 })
        expect(canvas.className).toContain('cursor-grabbing')
        fireEvent.pointerMove(canvas, { pointerId: 1, clientX: 160, clientY: 230 })
        fireEvent.pointerUp(canvas, { pointerId: 1, clientX: 160, clientY: 230 })
        expect(container.textContent).toContain('5s – 1.6m')
        expect(container.textContent).toContain('42 – 494 QPS')
    })

    it('中键（button=1）在画笔工具下也能平移', () => {
        const { canvas, container } = renderCanvas()
        fireEvent.pointerDown(canvas, { pointerId: 1, button: 1, clientX: 300, clientY: 200 })
        fireEvent.pointerMove(canvas, { pointerId: 1, clientX: 260, clientY: 200 })
        fireEvent.pointerUp(canvas, { pointerId: 1, clientX: 260, clientY: 200 })
        expect(container.textContent).toContain('5s – 1.6m')
    })

    it('按住空格临时平移，松开恢复画笔', () => {
        const { canvas, container } = renderCanvas()
        fireEvent.keyDown(window, { code: 'Space' })
        expect(canvas.className).toContain('cursor-grab')
        fireEvent.pointerDown(canvas, { pointerId: 1, button: 0, clientX: 200, clientY: 200 })
        fireEvent.pointerMove(canvas, { pointerId: 1, clientX: 160, clientY: 200 })
        fireEvent.pointerUp(canvas, { pointerId: 1, clientX: 160, clientY: 200 })
        expect(container.textContent).toContain('5s – 1.6m')
        fireEvent.keyUp(window, { code: 'Space' })
        expect(canvas.className).toContain('cursor-crosshair')
    })

    it('向左上拖出边界时相机钳制在 0', () => {
        const { canvas, container } = renderCanvas()
        fireEvent.click(screen.getByTitle('平移 (按住空格可临时使用)'))
        fireEvent.pointerDown(canvas, { pointerId: 1, button: 0, clientX: 200, clientY: 200 })
        fireEvent.pointerMove(canvas, { pointerId: 1, clientX: 700, clientY: 40 })
        fireEvent.pointerUp(canvas, { pointerId: 1, clientX: 700, clientY: 40 })
        expect(container.textContent).toContain('0s – 1.5m')
        expect(container.textContent).toContain('0 – 453 QPS')
    })

    it('放大/缩小按钮更新缩放百分比', () => {
        renderCanvas()
        fireEvent.click(screen.getByTitle('放大'))
        expect(screen.getByText('120%')).toBeInTheDocument()
        fireEvent.click(screen.getByTitle('缩小'))
        expect(screen.getByText('100%')).toBeInTheDocument()
    })

    it('滚轮在 plot 内缩放，plot 外不缩放', () => {
        const { canvas } = renderCanvas()
        fireEvent.wheel(canvas, { deltaY: -100, clientX: 372, clientY: 200 })
        expect(screen.getByText('115%')).toBeInTheDocument()
        fireEvent.wheel(canvas, { deltaY: 100, clientX: 30, clientY: 200 })
        expect(screen.getByText('115%')).toBeInTheDocument()
    })

    it('适配曲线：无曲线复位，有曲线自适应，回到原点复位', () => {
        const { canvas } = renderCanvas()
        fireEvent.click(screen.getByTitle('适配曲线'))
        expect(screen.getByText('100%')).toBeInTheDocument()
        drawStroke(canvas, [
            [100, 300],
            [200, 300],
            [300, 300],
            [400, 300],
            [500, 300],
        ])
        fireEvent.click(screen.getByTitle('适配曲线'))
        expect(screen.getByText('149%')).toBeInTheDocument()
        fireEvent.click(screen.getByTitle('回到原点 (0)'))
        expect(screen.getByText('100%')).toBeInTheDocument()
    })
})

describe('DrawCanvas 撤销/重做/清空/快捷键', () => {
    it('Ctrl+Z 撤销，Ctrl+Shift+Z 重做', () => {
        const { canvas, container } = renderCanvas()
        drawStroke(canvas, [
            [100, 300],
            [200, 300],
        ])
        expect(container.textContent).toContain('2 采样点')
        fireEvent.keyDown(window, { key: 'z', ctrlKey: true })
        expect(container.textContent).toContain('0 笔')
        expect(screen.getByTitle('撤销 (Ctrl/⌘ Z)')).toBeDisabled()
        fireEvent.keyDown(window, { key: 'Z', ctrlKey: true, shiftKey: true })
        expect(container.textContent).toContain('1 笔')
        expect(container.textContent).toContain('2 采样点')
        expect(screen.getByTitle('重做 (Ctrl/⌘ Shift Z)')).toBeDisabled()
    })

    it('输入框聚焦时 Ctrl+Z 不触发撤销（isTypingTarget 守卫）', () => {
        const { canvas, container } = renderCanvas()
        drawStroke(canvas, [
            [100, 300],
            [200, 300],
        ])
        const input = screen.getByLabelText('模板名称')
        fireEvent.keyDown(input, { key: 'z', ctrlKey: true })
        expect(container.textContent).toContain('1 笔')
    })

    it('P/H/0 快捷键切换工具与复位视图', () => {
        const { canvas } = renderCanvas()
        fireEvent.keyDown(window, { key: 'h' })
        expect(canvas.className).toContain('cursor-grab')
        fireEvent.keyDown(window, { key: 'p' })
        expect(canvas.className).toContain('cursor-crosshair')
        fireEvent.click(screen.getByTitle('放大'))
        expect(screen.getByText('120%')).toBeInTheDocument()
        fireEvent.keyDown(window, { key: '0' })
        expect(screen.getByText('100%')).toBeInTheDocument()
    })

    it('清空曲线后重做/撤销按钮均禁用（清空即清空 redo 栈，#185）', () => {
        const { canvas, container } = renderCanvas()
        drawStroke(canvas, [
            [100, 300],
            [200, 300],
        ])
        fireEvent.click(screen.getByTitle('清空曲线'))
        expect(container.textContent).toContain('0 笔')
        expect(container.textContent).toContain('0 采样点')
        expect(screen.getByTitle('清空曲线')).toBeDisabled()
        expect(screen.getByTitle('撤销 (Ctrl/⌘ Z)')).toBeDisabled()
        expect(screen.getByTitle('重做 (Ctrl/⌘ Shift Z)')).toBeDisabled()
    })
})

describe('DrawCanvas 保存与取消', () => {
    it('名称为空时提示且不调用 onSave', () => {
        const { onSave } = renderCanvas()
        fireEvent.click(screen.getByRole('button', { name: /保存模板/ }))
        expect(screen.getByText('请先为模板填写名称')).toBeInTheDocument()
        expect(onSave).not.toHaveBeenCalled()
    })

    it('有名称但曲线过短时提示', () => {
        const { canvas, onSave } = renderCanvas()
        drawStroke(canvas, [[100, 300]])
        fireEvent.change(screen.getByLabelText('模板名称'), { target: { value: 'tpl' } })
        fireEvent.click(screen.getByRole('button', { name: /保存模板/ }))
        expect(screen.getByText('请从左向右绘制一段更长的曲线')).toBeInTheDocument()
        expect(onSave).not.toHaveBeenCalled()
    })

    it('直线曲线保存：名称去空格，简化到端点，坐标 3/2 位小数', () => {
        const { canvas, onSave } = renderCanvas()
        drawStroke(canvas, [
            [100, 300],
            [200, 300],
            [300, 300],
            [400, 300],
            [500, 300],
        ])
        fireEvent.change(screen.getByLabelText('模板名称'), { target: { value: '  flat-curve  ' } })
        fireEvent.click(screen.getByRole('button', { name: /保存模板/ }))
        expect(onSave).toHaveBeenCalledTimes(1)
        const [name, points] = onSave.mock.calls[0] as [
            string,
            Array<{ x: number; y: number }>,
        ]
        expect(name).toBe('flat-curve')
        expect(points).toHaveLength(2)
        expect(points[0]).toEqual({ x: 3.5, y: 66.67 })
        expect(points[1]).toEqual({ x: 53.5, y: 66.67 })
        expect(points[1].x).toBeGreaterThan(points[0].x)
    })

    it('带拐点的曲线保存时保留拐点（简化不过度）', () => {
        const { canvas, onSave } = renderCanvas()
        drawStroke(canvas, [
            [100, 300],
            [200, 280],
            [300, 260],
            [400, 300],
            [500, 280],
        ])
        fireEvent.change(screen.getByLabelText('模板名称'), { target: { value: 'bump' } })
        fireEvent.click(screen.getByRole('button', { name: /保存模板/ }))
        expect(onSave).toHaveBeenCalledTimes(1)
        const points = onSave.mock.calls[0][1] as Array<{ x: number; y: number }>
        expect(points.length).toBeGreaterThan(2)
        for (let index = 1; index < points.length; index += 1) {
            expect(points[index].x).toBeGreaterThan(points[index - 1].x)
            expect(Number.isInteger(points[index].x * 1000)).toBe(true)
            expect(Number.isInteger(points[index].y * 100)).toBe(true)
        }
    })

    it('输入框回车直接保存', () => {
        const { canvas, onSave } = renderCanvas()
        drawStroke(canvas, [
            [100, 300],
            [200, 300],
        ])
        const input = screen.getByLabelText('模板名称')
        fireEvent.change(input, { target: { value: 'enter-tpl' } })
        fireEvent.keyDown(input, { key: 'Enter' })
        expect(onSave).toHaveBeenCalledTimes(1)
        expect(onSave.mock.calls[0][0]).toBe('enter-tpl')
    })

    it('取消按钮触发 onCancel', () => {
        const { onCancel } = renderCanvas()
        fireEvent.click(screen.getByRole('button', { name: '取消' }))
        expect(onCancel).toHaveBeenCalledTimes(1)
    })
})
