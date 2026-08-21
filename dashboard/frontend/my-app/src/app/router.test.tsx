import { describe, expect, it } from 'vitest'
import { router } from '@/app/router'

describe('app router', () => {
    it('根路由挂载 MainLayout 并注册全部子路由', () => {
        const root = router.routes[0]
        expect(root.path).toBe('/')
        expect(root.children?.length).toBe(8)
        const paths = root.children!.map((child) => child.path)
        expect(paths).toEqual(
            expect.arrayContaining(['observatory', 'config', 'traffic', 'guide']),
        )
        const redirects = root.children!.filter((child) => child.element && typeof child.element === 'object')
        // index/trace/monitor 重定向到 observatory
        expect(paths).toContain('*')
    })

    it('index 路由重定向到 observatory', () => {
        const root = router.routes[0]
        const index = root.children!.find((child) => child.index)
        expect(index).toBeDefined()
        expect(JSON.stringify(index!.element)).toContain('/observatory')
    })
})