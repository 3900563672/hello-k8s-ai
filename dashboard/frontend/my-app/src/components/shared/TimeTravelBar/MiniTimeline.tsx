import { TimelineChart } from './TimelineChart'

/**
 * 顶部栏中的高密度概览。缩放窗口与全屏探索器共享，
 * 因此用户在任一入口的缩放、平移和时间选择都会保持一致。
 */
export function MiniTimeline() {
    return <TimelineChart variant="mini" />
}
