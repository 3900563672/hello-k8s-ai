let fallbackSequence = 0

/**
 * 为请求去重和未保存的编辑草稿生成标识。
 * 业务数据与资源身份始终以 Dashboard Backend 为准。
 */
export function createClientId(prefix: string): string {
    const cryptoApi = globalThis.crypto
    if (cryptoApi?.randomUUID) {
        return `${prefix}-${cryptoApi.randomUUID()}`
    }

    if (cryptoApi?.getRandomValues) {
        const bytes = cryptoApi.getRandomValues(new Uint8Array(16))
        const value = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
        return `${prefix}-${value}`
    }

    fallbackSequence += 1
    return `${prefix}-${Date.now().toString(36)}-${fallbackSequence.toString(36)}`
}
