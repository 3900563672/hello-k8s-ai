import { lazy, Suspense, type ReactNode } from 'react'
import { createBrowserRouter, Navigate } from 'react-router-dom'
import { MainLayout } from '@/components/shared/Layout/MainLayout'
import {
    NotFoundPage,
    PageLoader,
    RouteErrorBoundary,
} from '@/components/shared/feedback/RouteFallbacks'

const ConfigPage = lazy(() =>
    import('@/components/features/config/ConfigPage').then((module) => ({
        default: module.ConfigPage,
    })),
)
const TrafficPage = lazy(() =>
    import('@/components/features/traffic/TrafficPage').then((module) => ({
        default: module.TrafficPage,
    })),
)
const DataOverviewPage = lazy(() =>
    import('@/components/features/trace/DataOverviewPage').then((module) => ({
        default: module.DataOverviewPage,
    })),
)
const MonitorPage = lazy(() =>
    import('@/components/features/monitor/MonitorPage').then((module) => ({
        default: module.MonitorPage,
    })),
)
const GuidePage = lazy(() =>
    import('@/components/features/guide/GuidePage').then((module) => ({
        default: module.GuidePage,
    })),
)

const deferred = (page: ReactNode) => (
    <Suspense fallback={<PageLoader />}>{page}</Suspense>
)

export const router = createBrowserRouter([
    {
        path: '/',
        element: <MainLayout />,
        errorElement: <RouteErrorBoundary />,
        children: [
            { index: true, element: <Navigate to="/config" replace /> },
            { path: 'config', element: deferred(<ConfigPage />) },
            { path: 'traffic', element: deferred(<TrafficPage />) },
            { path: 'trace', element: deferred(<DataOverviewPage />) },
            { path: 'monitor', element: deferred(<MonitorPage />) },
            { path: 'guide', element: deferred(<GuidePage />) },
            { path: '*', element: <NotFoundPage /> },
        ],
    },
])
