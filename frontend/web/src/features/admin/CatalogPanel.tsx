import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { adminListImages, adminListProducts } from '@/shared/api/admin'
import { formatPrice } from '@/shared/lib/format'
import { formatDateTime } from '@/shared/lib/orderStatus'
import { SmartImage } from '@/shared/ui/SmartImage'
import type { AdminContext } from './AdminPage'
import { JsonView, Pager, QueryState } from './shared'
import styles from './AdminData.module.css'

const pageSize = 20

export function CatalogPanel({ context }: { context: AdminContext }) {
  const { readKey } = context
  const [productOffset, setProductOffset] = useState(0)
  const [imageOffset, setImageOffset] = useState(0)
  const products = useQuery({
    queryKey: ['admin', 'products', readKey, productOffset],
    queryFn: ({ signal }) => adminListProducts(readKey, productOffset, pageSize, signal),
    enabled: readKey.length > 0,
  })
  const images = useQuery({
    queryKey: ['admin', 'images', readKey, imageOffset],
    queryFn: ({ signal }) => adminListImages(readKey, imageOffset, 24, signal),
    enabled: readKey.length > 0,
  })
  const productList = products.data?.products ?? []
  const imageList = images.data?.images ?? []
  return (
    <div className={styles.panel}>
      <section className={styles.detail}>
        <h3 className={styles.subTitle}>Products ({products.data?.total ?? 0})</h3>
        <QueryState
          isLoading={products.isLoading}
          error={products.error}
          empty={productList.length === 0}
          emptyText="No products yet. Add some in the Products tab."
        >
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Product</th>
                  <th>Variants (SKU · price · stock)</th>
                  <th>Stock</th>
                  <th>Updated</th>
                </tr>
              </thead>
              <tbody>
                {productList.map((product) => (
                  <tr key={product.id}>
                    <td>
                      <strong>{product.name}</strong>
                      <div className={styles.mono}>{product.id}</div>
                      <div className={styles.muted}>{(product.categories ?? []).join(', ')}</div>
                    </td>
                    <td>
                      {product.variants.map((variant) => (
                        <div key={variant.id}>
                          <span className={styles.mono}>{variant.sku}</span> ·{' '}
                          {formatPrice(variant.selling_price, variant.currency)} ·{' '}
                          <span
                            style={{
                              color: variant.stock_units === 0 ? 'var(--danger)' : undefined,
                            }}
                          >
                            {variant.stock_units} left
                          </span>
                        </div>
                      ))}
                    </td>
                    <td>{product.total_stock}</td>
                    <td>{formatDateTime(product.updated_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <JsonView value={productList} />
        </QueryState>
        <Pager
          offset={productOffset}
          limit={pageSize}
          total={products.data?.total ?? 0}
          onChange={setProductOffset}
        />
      </section>
      <section className={styles.detail}>
        <h3 className={styles.subTitle}>Images ({images.data?.total ?? 0})</h3>
        <QueryState
          isLoading={images.isLoading}
          error={images.error}
          empty={imageList.length === 0}
          emptyText="No images uploaded yet."
        >
          <div className={styles.thumbGrid}>
            {imageList.map((image) => (
              <div key={image.id} className={styles.thumbCard}>
                <SmartImage src={image.id} alt={image.file_name || image.id} ratio="1 / 1" />
                <span className={styles.mono}>{image.id}</span>
                <span className={styles.muted}>
                  {image.width}×{image.height} · {Math.round(image.size / 1024)} KB
                </span>
              </div>
            ))}
          </div>
        </QueryState>
        <Pager
          offset={imageOffset}
          limit={24}
          total={images.data?.total ?? 0}
          onChange={setImageOffset}
        />
      </section>
    </div>
  )
}
