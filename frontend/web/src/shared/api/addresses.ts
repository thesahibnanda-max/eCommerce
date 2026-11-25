import { request } from './client'
import type { Address, AddressInput } from './types'

type AddressResponse = { address: Address }

export function listAddresses(signal?: AbortSignal): Promise<Address[]> {
  return request<{ addresses: Address[] }>('/addresses', { auth: true, signal }).then(
    (data) => data.addresses,
  )
}

export function createAddress(input: AddressInput): Promise<Address> {
  return request<AddressResponse>('/addresses', { method: 'POST', auth: true, body: input }).then(
    (data) => data.address,
  )
}

export function updateAddress(id: string, input: AddressInput): Promise<Address> {
  return request<AddressResponse>(`/addresses/${encodeURIComponent(id)}`, {
    method: 'PUT',
    auth: true,
    body: input,
  }).then((data) => data.address)
}

export function deleteAddress(id: string): Promise<string> {
  return request<{ id: string }>(`/addresses/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    auth: true,
  }).then((data) => data.id)
}

export function setDefaultAddress(id: string): Promise<Address> {
  return request<AddressResponse>(`/addresses/${encodeURIComponent(id)}/default`, {
    method: 'POST',
    auth: true,
  }).then((data) => data.address)
}
