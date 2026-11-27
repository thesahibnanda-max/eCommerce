export type Variant = {
  id: string
  sku: string
  attributes?: Record<string, string>
  shown_price: number
  selling_price: number
  currency: string
  stock_units: number
  image_urls?: string[]
}

export type Product = {
  id: string
  name: string
  description: string
  metadata?: Record<string, unknown>
  variants: Variant[]
  categories?: string[]
  created_at: string
  updated_at: string
}

export type CategoryCount = { category: string; count: number }

export type ProductPage = { products: Product[]; total: number }

export type SearchResult = ProductPage & {
  category_facets: CategoryCount[]
  suggestion?: string
  suggested_products?: Product[]
}

export type SearchSort = 'relevance' | 'price_asc' | 'price_desc' | 'newest'

export type SearchParams = {
  text: string
  categories: string[]
  minPrice: number | null
  maxPrice: number | null
  inStockOnly: boolean
  sort: SearchSort
}

export type Session = { id: string; access_token: string; refresh_token: string }

export type CartLine = {
  variant_id: string
  product_id?: string
  name?: string
  sku?: string
  attributes?: Record<string, string>
  image_url?: string
  shown_price: number
  selling_price: number
  currency?: string
  stock_units: number
  quantity: number
  available: boolean
  added_at: string
}

export type CartTotal = { currency: string; shown_price: number; selling_price: number }

export type Cart = {
  items: CartLine[]
  item_count: number
  totals: CartTotal[]
  updated_at: string
}

export type CartItemInput = { variant_id: string; quantity: number }

export type VariantInput = {
  id?: string
  sku: string
  attributes?: Record<string, string>
  shown_price: number
  selling_price: number
  currency: string
  stock_units: number
  image_urls?: string[]
}

export type ProductInput = {
  name: string
  description: string
  metadata?: Record<string, unknown>
  variants: VariantInput[]
  categories?: string[]
}

export type ProductUpdateInput = ProductInput & { id: string }

export type ImageInfo = {
  id: string
  file_name: string
  content_type: string
  width: number
  height: number
  size: number
  original_size: number
  metadata: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

export type Address = {
  id: string
  label: string
  full_name: string
  phone_number: string
  address_line1: string
  address_line2: string
  landmark: string
  city: string
  state: string
  state_code: string
  country: string
  pincode: string
  is_default: boolean
  created_at: string
  updated_at: string
}

export type AddressInput = {
  label: string
  full_name: string
  phone_number: string
  address_line1: string
  address_line2: string
  landmark: string
  city: string
  pincode: string
  is_default: boolean
}

export type QuoteLine = {
  variant_id: string
  product_id: string
  product_name: string
  sku: string
  attributes: Record<string, string> | null
  image_url?: string
  unit_price: number
  quantity: number
  line_total: number
  available: boolean
  stock_units: number
}

export type Quote = {
  lines: QuoteLine[]
  item_count: number
  subtotal: number
  shipping_fee: number
  total_payable: number
  free_shipping_above: number
  currency: string
  ready: boolean
  problems: string[]
  payment_provider: string
}

export type PaymentAction = {
  type: 'SHOW_QR' | 'REDIRECT' | string
  redirect_url?: string
  qr_image_data_uri?: string
  upi_id?: string
  upi_intent_url?: string
}

export type PaymentEvent = {
  source: string
  kind: string
  from_state?: string
  to_state?: string
  data?: Record<string, unknown>
  created_at: string
}

export type OrderPayment = {
  id: string
  order_id: string
  user_id?: string
  provider: string
  state: string
  amount: number
  currency: string
  checkout_url?: string
  gateway_reference?: string
  gateway_payment_id?: string
  proof_transaction_id?: string
  proof_submitted_at?: string
  has_proof_image: boolean
  failure_reason?: string
  metadata?: Record<string, unknown>
  events?: PaymentEvent[]
  action?: PaymentAction
  created_at: string
  updated_at: string
}

export type OrderItem = {
  product_id: string
  product_variant_id: string
  sku: string
  product_name: string
  attributes: Record<string, string> | null
  unit_price: number
  quantity: number
  line_total: number
  image_url?: string
}

export type OrderAddress = {
  address_id: string
  full_name: string
  phone_number: string
  address_line1: string
  address_line2: string
  landmark: string
  city: string
  state: string
  state_code: string
  pincode: string
  country: string
}

export type StateChange = {
  from_state: string
  to_state: string
  reason: string
  actor: string
  created_at: string
}

export type Order = {
  id: string
  user_id?: string
  state: string
  currency: string
  subtotal: number
  shipping_fee: number
  discount: number
  total_payable: number
  item_count: number
  items: OrderItem[]
  shipping_address: OrderAddress
  payment: OrderPayment | null
  tracking: OrderTracking | null
  steps: OrderStep[]
  closed: boolean
  payments?: OrderPayment[]
  history?: StateChange[]
  allowed_transitions?: string[]
  metadata?: Record<string, unknown>
  can_pay: boolean
  can_cancel: boolean
  can_submit_proof: boolean
  created_at: string
  updated_at: string
}

export type OrderTracking = { courier: string; number: string; url: string }

export type OrderStep = {
  key: string
  label: string
  done: boolean
  current: boolean
  at: string | null
}

export type OrderPage = { orders: Order[]; next_cursor: string }

export type PaymentPage = { payments: OrderPayment[]; next_cursor: string }

export type AdminOverview = {
  users: number
  products: number
  images: number
  carts: number
  orders_by_state: Record<string, number>
  payments_by_state: Record<string, number>
  payments_awaiting_review: number
  orders_to_fulfil: number
  payment_provider: string
}

export type AdminUser = {
  id: string
  name: string
  email: string
  phone_number: string
  is_email_validated: boolean
  is_phone_number_validated: boolean
  metadata: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

export type AdminCartLine = {
  variant_id: string
  product_id?: string
  product_name?: string
  sku?: string
  attributes?: Record<string, string>
  selling_price: number
  stock_units: number
  quantity: number
  exists: boolean
  added_at: string
}

export type AdminCart = {
  user_id: string
  lines: AdminCartLine[]
  item_count: number
  value: number
  updated_at: string
}

export type AdminVariant = {
  id: string
  sku: string
  attributes: Record<string, string> | null
  shown_price: number
  selling_price: number
  currency: string
  stock_units: number
  image_urls: string[] | null
}

export type AdminProduct = {
  id: string
  name: string
  description: string
  categories: string[] | null
  metadata: Record<string, unknown> | null
  variants: AdminVariant[]
  total_stock: number
  created_at: string
  updated_at: string
}

export type AdminImage = {
  id: string
  file_name: string
  content_type: string
  width: number
  height: number
  size: number
  original_size: number
  metadata: Record<string, unknown> | null
  created_at: string
}

export type AdminUserDetail = {
  user: AdminUser
  addresses: Address[]
  cart: AdminCart
  orders: Order[]
}

export type SnapshotDatabaseResult = {
  database: string
  url?: string
  sheets: number
  rows: number
  error?: string
}

export type SnapshotResult = {
  started_at: string
  finished_at: string
  databases: SnapshotDatabaseResult[]
  failed: number
}

export type SnapshotStatus = {
  configured: boolean
  interval?: string
  running: boolean
  last?: SnapshotResult
  result?: SnapshotResult
}
