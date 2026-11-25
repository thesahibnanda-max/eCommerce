export type StatusTone = 'pink' | 'butter' | 'mint' | 'lilac' | 'sky' | 'peach' | 'grey'

type StatusInfo = { label: string; tone: StatusTone; hint: string }

const orderStatuses: Record<string, StatusInfo> = {
  CREATED: { label: 'Awaiting payment', tone: 'butter', hint: 'Pay to confirm your order.' },
  PROCESSING: { label: 'Checking payment', tone: 'sky', hint: 'We are verifying your payment.' },
  ON_HOLD: { label: 'On hold', tone: 'peach', hint: 'We need a moment to look into this.' },
  APPROVED: {
    label: 'Confirmed',
    tone: 'mint',
    hint: 'Payment received. We are getting your order ready.',
  },
  PACKED: { label: 'Packed', tone: 'mint', hint: 'Packed with love and ready to ship.' },
  STARTED_SHIPPING: { label: 'Shipped', tone: 'lilac', hint: 'Your order is on its way.' },
  OUT_FOR_DELIVERY: {
    label: 'Out for delivery',
    tone: 'sky',
    hint: 'Arriving today. Keep your phone handy.',
  },
  DELIVERED: { label: 'Delivered', tone: 'mint', hint: 'Enjoy your new pieces!' },
  CANCELLED: { label: 'Cancelled', tone: 'grey', hint: 'This order was cancelled.' },
  RETURNED: { label: 'Returned', tone: 'grey', hint: 'This order was returned.' },
  REFUNDED: { label: 'Refunded', tone: 'grey', hint: 'Your refund has been processed.' },
  FAILED: { label: 'Payment failed', tone: 'pink', hint: 'We could not confirm the payment.' },
  DITCHED: { label: 'Closed', tone: 'grey', hint: 'This order was closed.' },
}

const paymentStatuses: Record<string, StatusInfo> = {
  CREATED: { label: 'Starting', tone: 'butter', hint: 'Setting up your payment.' },
  AWAITING_PAYMENT: { label: 'Not paid yet', tone: 'butter', hint: 'Complete the payment.' },
  VERIFYING: { label: 'Under review', tone: 'sky', hint: 'We are checking your payment.' },
  SUCCEEDED: { label: 'Paid', tone: 'mint', hint: 'Payment received.' },
  FAILED: { label: 'Failed', tone: 'pink', hint: 'Payment was not confirmed.' },
  EXPIRED: { label: 'Expired', tone: 'grey', hint: 'The payment window closed.' },
}

function lookup(table: Record<string, StatusInfo>, state: string): StatusInfo {
  return table[state] ?? { label: state.replaceAll('_', ' ').toLowerCase(), tone: 'grey', hint: '' }
}

export function orderStatus(state: string): StatusInfo {
  return lookup(orderStatuses, state)
}

export function paymentStatus(state: string): StatusInfo {
  return lookup(paymentStatuses, state)
}

export const orderStates = Object.keys(orderStatuses)

export const paymentStates = Object.keys(paymentStatuses)

export function shortOrderID(id: string): string {
  return id.length >= 26 ? id.slice(18, 26).toUpperCase() : id.toUpperCase()
}

export function formatDateTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('en-IN', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}
