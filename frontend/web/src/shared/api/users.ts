import { request } from './client'
import type { Session } from './types'

export type Contact = { email?: string; phone_number?: string }

export type SignUpInput = { name: string; email: string; phone_number: string; password: string }

export function signUp(input: SignUpInput): Promise<Session> {
  return request<Session>('/users/signup', { method: 'POST', body: input })
}

export function logIn(input: Contact & { password: string }): Promise<Session> {
  return request<Session>('/users/login', { method: 'POST', body: input })
}

export function startEmailValidation(): Promise<{ id: string; email: string }> {
  return request('/users/email/validation', { method: 'POST', auth: true })
}

export function validateEmail(otp: string): Promise<{ id: string; is_validated: boolean }> {
  return request('/users/email/validate', { method: 'POST', auth: true, body: { otp } })
}

export type Me = {
  id: string
  name: string
  email: string
  phone_number: string
  is_validated: boolean
  is_email_validated: boolean
  is_phone_number_validated: boolean
}

export function fetchMe(signal?: AbortSignal): Promise<Me> {
  return request<Me>('/users/me', { auth: true, signal })
}
