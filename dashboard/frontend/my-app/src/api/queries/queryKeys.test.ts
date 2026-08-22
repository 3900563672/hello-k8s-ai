import { describe, expect, it } from 'vitest'
import { aiopsQueryKeys } from '@/api/queries/aiopsQueries'
import { configKeys } from '@/api/queries/configQueries'
import { traceQueryKeys } from '@/api/queries/traceQueries'

/**
 * DATA_FLOW.md 要求：query key 必须包含所有影响结果的参数（at、tenant、metric/trace filter），
 * 否则 latest 与 historical 缓存可能串用。这里把该不变量固化为测试。
 */
describe('query keys 不变量', () => {
    it('trace overview key 区分模式/快照/筛选参数', () => {
        const latest = traceQueryKeys.overview({
            effectiveAt: '2026-08-12T12:00:00.000Z',
            snapshotId: null,
            mode: 'latest',
            revision: 0,
            tenantId: 'tenant-a',
            modelId: 'model-b',
            instanceId: undefined,
        })
        expect(latest).toEqual(['trace', 'overview', 'latest', 'latest', 'tenant-a', 'model-b', null])

        const historical = traceQueryKeys.overview({
            effectiveAt: '2026-08-12T11:00:00.000Z',
            snapshotId: 'snap-1',
            mode: 'historical',
            revision: 1,
            tenantId: 'tenant-a',
            modelId: undefined,
            instanceId: undefined,
        })
        expect(historical).toEqual(['trace', 'overview', 'historical', 'snap-1', 'tenant-a', null, null])
        expect(latest).not.toEqual(historical)
    })

    it('trace detail/segment key 包含 traceId 与时间窗参数', () => {
        expect(traceQueryKeys.detail('trace-1')).toEqual(['trace', 'detail', 'trace-1'])
        expect(traceQueryKeys.detail(null)).toEqual(['trace', 'detail', null])
        const segment = traceQueryKeys.segment({
            start: '2026-08-12T12:00:00.000Z',
            end: '2026-08-12T12:05:00.000Z',
            tenantId: 'tenant-a',
        })
        expect(segment).toEqual(['trace', 'segment', '2026-08-12T12:00:00.000Z', '2026-08-12T12:05:00.000Z', 'tenant-a', null, null])
    })

    it('aiops keys 区分状态过滤与分析对象', () => {
        expect(aiopsQueryKeys.all).toEqual(['aiops'])
        expect(aiopsQueryKeys.analyses('completed')).toEqual(['aiops', 'analyses', 'completed'])
        expect(aiopsQueryKeys.analyses(undefined)).toEqual(['aiops', 'analyses', null])
        expect(aiopsQueryKeys.detail('segment-1')).toEqual(['aiops', 'detail', 'segment-1'])
        expect(aiopsQueryKeys.limits).toEqual(['aiops', 'limits'])
    })

    it('config keys 区分配置版本（latest / 快照）', () => {
        expect(configKeys.models()).toEqual(['config', 'models', 'latest'])
        expect(configKeys.models('snap-1')).toEqual(['config', 'models', 'snap-1'])
        expect(configKeys.configuration()).toEqual(['config', 'configuration'])
        expect(configKeys.tenantDetail('tenant-a')).toEqual(['config', 'tenants', 'latest', 'tenant-a'])
    })
})
