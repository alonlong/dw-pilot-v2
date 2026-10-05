const hoursFmt = new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 })

export const formatHours = (h: number) => hoursFmt.format(h)

const timeFmt = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
})

/** RFC 3339 → local "2026/09/02 18:05". */
export const formatTime = (iso: string) => timeFmt.format(new Date(iso))
