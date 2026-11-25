import { request } from './client'

export type InfoSection = { heading: string; paragraphs?: string[]; bullets?: string[] }

export type InfoContact = {
  company_name: string
  email: string
  phone: string
  address: string
  grievance_officer: string
}

export type InfoPage = {
  slug: string
  title: string
  summary: string
  effective_date: string
  sections: InfoSection[]
}

export type InfoPageResponse = { page: InfoPage; contact: InfoContact }

export const contactSlug = 'contact-us'

export const infoLinks = [
  { slug: contactSlug, label: 'Contact Us' },
  { slug: 'terms-and-conditions', label: 'Terms & Conditions' },
  { slug: 'privacy-policy', label: 'Privacy Policy' },
  { slug: 'refund-and-return-policy', label: 'Refund & Return Policy' },
  { slug: 'shipping-policy', label: 'Shipping Policy' },
] as const

export function getInfoPage(slug: string, signal?: AbortSignal): Promise<InfoPageResponse> {
  return request<InfoPageResponse>(
    `/info/pages/${encodeURIComponent(slug)}`,
    signal ? { signal } : {},
  )
}
