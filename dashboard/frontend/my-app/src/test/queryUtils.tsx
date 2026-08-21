import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { useTimeStore } from '@/stores/timeSlice'

/** 测试专用 QueryClient：关闭重试与重试延迟，避免 waitFor 超时。 */
export const createQueryClient = (): QueryClient =>
    new QueryClient({
        defaultOptions: {
            queries: { retry: false, retryDelay: 0 },
            mutations: { retry: false },
        },
    })

export const wrapperFor =
    (client: QueryClient) =>
    ({ children }: { children: ReactNode }) =>
        <QueryClientProvider client={client}>{children}</QueryClientProvider>

/** 把时间回放 store 复位为 latest 默认态（queries 测试互相隔离）。 */
export const resetReplayStore = (): void => {
    useTimeStore.setState({
        timestamp: new Date(0).toISOString(),
        selectedSnapshotId: null,
        mode: 'latest',
        revision: 0,
        snapshots: [],
    })
}