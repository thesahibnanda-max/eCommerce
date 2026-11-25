import { lazy, Suspense, useEffect } from 'react'
import { Route, Routes } from 'react-router'
import { restoreSession } from '@/shared/auth/session'
import { Toaster } from '@/shared/ui/Toaster'
import { HomePage } from '@/features/home/HomePage'
import { ErrorBoundary } from './ErrorBoundary'
import { PageFallback } from './layout/PageFallback'
import { ScrollToTop } from './layout/ScrollToTop'
import { Shell } from './layout/Shell'

const SearchPage = lazy(() => import('@/features/search/SearchPage'))
const CategoryPage = lazy(() => import('@/features/category/CategoryPage'))
const ProductPage = lazy(() => import('@/features/product/ProductPage'))
const CartPage = lazy(() => import('@/features/cart/CartPage'))
const CheckoutPage = lazy(() => import('@/features/checkout/CheckoutPage'))
const AccountPage = lazy(() => import('@/features/account/AccountPage'))
const AdminPage = lazy(() => import('@/features/admin/AdminPage'))
const InfoPage = lazy(() => import('@/features/info/InfoPage'))
const OrderPage = lazy(() => import('@/features/orders/OrderPage'))
const PaymentReturnPage = lazy(() => import('@/features/orders/PaymentReturnPage'))
const NotFoundPage = lazy(() => import('./NotFoundPage'))

export function App() {
  useEffect(() => {
    void restoreSession()
  }, [])
  return (
    <ErrorBoundary>
      <ScrollToTop />
      <Suspense fallback={<PageFallback />}>
        <Routes>
          <Route path="/admin" element={<AdminPage />} />
          <Route element={<Shell />}>
            <Route index element={<HomePage />} />
            <Route path="search" element={<SearchPage />} />
            <Route path="c/:category" element={<CategoryPage />} />
            <Route path="p/:id" element={<ProductPage />} />
            <Route path="cart" element={<CartPage />} />
            <Route path="checkout" element={<CheckoutPage />} />
            <Route path="account" element={<AccountPage />} />
            <Route path="orders/:id" element={<OrderPage />} />
            <Route path="payments/return" element={<PaymentReturnPage />} />
            <Route path="info/:slug" element={<InfoPage />} />
            <Route path="*" element={<NotFoundPage />} />
          </Route>
        </Routes>
      </Suspense>
      <Toaster />
    </ErrorBoundary>
  )
}
