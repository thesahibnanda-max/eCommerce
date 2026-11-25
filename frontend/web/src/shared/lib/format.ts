const formatters = new Map<string, Intl.NumberFormat>()

function formatterFor(currency: string): Intl.NumberFormat {
  const key = currency.toUpperCase()
  const cached = formatters.get(key)
  if (cached) return cached
  let created: Intl.NumberFormat
  try {
    created = new Intl.NumberFormat(key === 'INR' ? 'en-IN' : undefined, {
      style: 'currency',
      currency: key,
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    })
  } catch {
    created = new Intl.NumberFormat(undefined, { style: 'decimal', maximumFractionDigits: 0 })
  }
  formatters.set(key, created)
  return created
}

export function formatPrice(amount: number, currency: string): string {
  const formatter = formatterFor(currency)
  const text = formatter.format(amount)
  return formatter.resolvedOptions().style === 'currency'
    ? text
    : `${text} ${currency.toUpperCase()}`
}

export function isWholeAmount(value: number): boolean {
  return Number.isSafeInteger(value) && value >= 0
}

export function discountPercent(shown: number, selling: number): number {
  if (shown <= 0 || selling >= shown) return 0
  return Math.round(((shown - selling) / shown) * 100)
}

export function titleCase(value: string): string {
  return value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ')
}
