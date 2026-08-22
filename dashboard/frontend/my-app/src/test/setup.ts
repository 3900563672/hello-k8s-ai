import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";

class ResizeObserverStub {
    observe() {}
    unobserve() {}
    disconnect() {}
}

globalThis.ResizeObserver ??= ResizeObserverStub as unknown as typeof ResizeObserver;

// jsdom 不自动推进 rAF；组件用 rAF 复位状态（如 TimelineChart 的 applyingOptionRef）时必须同步执行
globalThis.requestAnimationFrame = ((callback: FrameRequestCallback) => {
    callback(0)
    return 0
}) as typeof requestAnimationFrame
globalThis.cancelAnimationFrame = (() => {}) as typeof cancelAnimationFrame

// radix-ui Select 打开时对候选元素调用 scrollIntoView（jsdom 缺失）
if (typeof Element !== "undefined" && typeof Element.prototype.scrollIntoView !== "function") {
    Element.prototype.scrollIntoView = () => {}
}

// radix-ui（Select/Slider 等）在 jsdom 下依赖 pointer capture API（api 测试跑 node 环境，需守卫）
if (typeof Element !== "undefined") {
    const elementProto = Element.prototype as unknown as Record<string, unknown>
    if (typeof elementProto.hasPointerCapture !== "function") {
        elementProto.hasPointerCapture = () => false
        elementProto.setPointerCapture = () => {}
        elementProto.releasePointerCapture = () => {}
    }
}

afterEach(() => {
    cleanup();
});
