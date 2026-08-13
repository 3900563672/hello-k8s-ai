export function formatUtcTimestamp(timestamp: string | number, includeMs = false) {
    const value = new Date(timestamp)
    if (Number.isNaN(value.getTime())) return 'Invalid timestamp'
    const iso = value.toISOString()
    return includeMs ? iso.replace('T', ' ').replace('Z', '') : iso.slice(0, 19).replace('T', ' ')
}
