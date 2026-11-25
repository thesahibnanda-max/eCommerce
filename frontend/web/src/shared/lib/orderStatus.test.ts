import { describe, expect, it } from 'vitest'
import { formatDateTime, orderStatus, paymentStatus, shortOrderID } from './orderStatus'

describe('orderStatus', () => {
  it('maps known order and payment states to friendly labels', () => {
    expect(orderStatus('APPROVED').label).toBe('Confirmed')
    expect(orderStatus('CREATED').tone).toBe('butter')
    expect(paymentStatus('VERIFYING').label).toBe('Under review')
    expect(paymentStatus('SUCCEEDED').tone).toBe('mint')
    expect(orderStatus('PACKED').label).toBe('Packed')
    expect(orderStatus('OUT_FOR_DELIVERY').label).toBe('Out for delivery')
  })

  it('falls back gracefully for unknown states', () => {
    expect(orderStatus('SOMETHING_NEW')).toEqual({ label: 'something new', tone: 'grey', hint: '' })
  })

  it('shortens order ids to the random part of the ulid', () => {
    expect(shortOrderID('01M87V2A0XBZ8MYWJRKS46CKDJ-01a20fb1-281d-78ae-a6a5-b318199ff3b7')).toBe(
      'KS46CKDJ',
    )
    expect(shortOrderID('abc')).toBe('ABC')
  })

  it('keeps unparseable dates as they are', () => {
    expect(formatDateTime('not a date')).toBe('not a date')
    expect(formatDateTime('2026-11-25T04:03:18Z')).toMatch(/2026/)
  })
})
