import { Plus, Trash2 } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { createProduct, createProducts, updateProduct, updateProducts } from '@/shared/api/admin'
import { getProduct } from '@/shared/api/catalog'
import type { ProductInput } from '@/shared/api/types'
import { Button } from '@/shared/ui/Button'
import { TextArea, TextField } from '@/shared/ui/TextField'
import type { AdminContext } from './AdminPage'
import {
  emptyVariant,
  parseProductUpdates,
  parseProducts,
  splitList,
  toVariantInput,
  type VariantDraft,
} from './parse'
import styles from './AdminPage.module.css'

const batchExample = `{"products":[{"name":"Indigo Kurta","description":"Hand block printed","categories":["kurtas"],"variants":[{"sku":"IK-M","attributes":{"size":"M"},"shown_price":1999,"selling_price":1499,"currency":"INR","stock_units":5,"image_urls":["/v1/images/<image-id>"]}]}]}`

export function ProductsPanel({ context }: { context: AdminContext }) {
  const { run, busy } = context
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [categories, setCategories] = useState('')
  const [variants, setVariants] = useState<VariantDraft[]>([emptyVariant()])
  const [formError, setFormError] = useState<string | null>(null)
  const [batch, setBatch] = useState('')
  const [editID, setEditID] = useState('')
  const [editJSON, setEditJSON] = useState('')
  const [batchUpdate, setBatchUpdate] = useState('')

  const updateVariant = (index: number, patch: Partial<VariantDraft>) =>
    setVariants((items) => items.map((item, i) => (i === index ? { ...item, ...patch } : item)))

  const create = async (event: FormEvent) => {
    event.preventDefault()
    setFormError(null)
    let product: ProductInput
    try {
      product = {
        name: name.trim(),
        description: description.trim(),
        variants: variants.map(toVariantInput),
      }
      const list = splitList(categories)
      if (list.length > 0) product.categories = list
    } catch (caught) {
      setFormError(caught instanceof Error ? caught.message : 'Invalid product')
      return
    }
    const result = await run(`Create ${product.name || 'product'}`, (key) =>
      createProduct(key, product),
    )
    if (result) {
      setName('')
      setDescription('')
      setCategories('')
      setVariants([emptyVariant()])
    }
  }

  const load = async () => {
    const id = editID.trim()
    if (!id) return
    const product = await run(`Load ${id}`, () => getProduct(id))
    if (product && typeof product === 'object') {
      const {
        name: productName,
        description: productDescription,
        metadata,
        categories: productCategories,
        variants: productVariants,
      } = product as ProductInput
      setEditJSON(
        JSON.stringify(
          {
            name: productName,
            description: productDescription,
            metadata,
            categories: productCategories,
            variants: productVariants,
          },
          null,
          2,
        ),
      )
    }
  }

  return (
    <div className={styles.panel}>
      <form className={styles.card} onSubmit={create}>
        <h2 className={styles.sectionTitle}>New product</h2>
        <TextField label="Name" value={name} onChange={(e) => setName(e.target.value)} />
        <TextArea
          label="Description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          style={{ minHeight: 90, fontFamily: 'inherit' }}
        />
        <TextField
          label="Categories"
          hint="Comma separated, e.g. kurtas, festive"
          value={categories}
          onChange={(e) => setCategories(e.target.value)}
        />
        {variants.map((variant, index) => (
          <fieldset key={index} className={styles.variant}>
            <legend>Variant {index + 1}</legend>
            <TextField
              label="SKU"
              value={variant.sku}
              onChange={(e) => updateVariant(index, { sku: e.target.value })}
            />
            <TextField
              label="Attributes"
              hint="key=value, comma separated"
              value={variant.attributes}
              onChange={(e) => updateVariant(index, { attributes: e.target.value })}
            />
            <div className={styles.grid2}>
              <TextField
                label="MRP (₹)"
                inputMode="numeric"
                value={variant.shownPrice}
                onChange={(e) => updateVariant(index, { shownPrice: e.target.value })}
              />
              <TextField
                label="Selling price (₹)"
                inputMode="numeric"
                hint="Blank = MRP"
                value={variant.sellingPrice}
                onChange={(e) => updateVariant(index, { sellingPrice: e.target.value })}
              />
              <TextField
                label="Currency"
                value={variant.currency}
                onChange={(e) => updateVariant(index, { currency: e.target.value })}
              />
              <TextField
                label="Stock"
                inputMode="numeric"
                value={variant.stock}
                onChange={(e) => updateVariant(index, { stock: e.target.value })}
              />
            </div>
            <TextField
              label="Images"
              hint="Image IDs or URLs, comma separated"
              value={variant.images}
              onChange={(e) => updateVariant(index, { images: e.target.value })}
            />
            {variants.length > 1 && (
              <Button
                variant="danger"
                icon={<Trash2 size={16} />}
                onClick={() => setVariants((items) => items.filter((_, i) => i !== index))}
              >
                Remove variant
              </Button>
            )}
          </fieldset>
        ))}
        <Button
          variant="ghost"
          icon={<Plus size={16} />}
          onClick={() => setVariants((items) => [...items, emptyVariant()])}
        >
          Add variant
        </Button>
        {formError && <p className={styles.error}>{formError}</p>}
        <Button type="submit" variant="accent" block loading={busy}>
          Create product
        </Button>
      </form>

      <div className={styles.card}>
        <h2 className={styles.sectionTitle}>Batch create (JSON)</h2>
        <TextArea
          label="Products"
          hint="Prices are whole rupees."
          placeholder={batchExample}
          value={batch}
          onChange={(e) => setBatch(e.target.value)}
        />
        <Button
          variant="accent"
          loading={busy}
          disabled={!batch.trim()}
          onClick={() =>
            void run('Batch create', (key) => createProducts(key, parseProducts(batch)))
          }
        >
          Create all
        </Button>
      </div>

      <div className={styles.card}>
        <h2 className={styles.sectionTitle}>Edit product</h2>
        <div className={styles.inline}>
          <TextField
            label="Product ID"
            value={editID}
            onChange={(e) => setEditID(e.target.value)}
          />
          <Button variant="ghost" onClick={() => void load()} disabled={!editID.trim()}>
            Load
          </Button>
        </div>
        {editJSON && (
          <>
            <TextArea
              label="Product JSON"
              value={editJSON}
              onChange={(e) => setEditJSON(e.target.value)}
              style={{ minHeight: 260 }}
            />
            <Button
              variant="accent"
              loading={busy}
              onClick={() =>
                void run(`Update ${editID.trim()}`, (key) => {
                  const parsed = parseProducts(`[${editJSON}]`)[0]
                  if (!parsed) throw new Error('Nothing to save')
                  return updateProduct(key, { ...parsed, id: editID.trim() })
                })
              }
            >
              Save changes
            </Button>
          </>
        )}
      </div>

      <div className={styles.card}>
        <h2 className={styles.sectionTitle}>Batch update (JSON)</h2>
        <TextArea
          label="Products with id"
          value={batchUpdate}
          onChange={(e) => setBatchUpdate(e.target.value)}
        />
        <Button
          variant="accent"
          loading={busy}
          disabled={!batchUpdate.trim()}
          onClick={() =>
            void run('Batch update', (key) => updateProducts(key, parseProductUpdates(batchUpdate)))
          }
        >
          Update all
        </Button>
      </div>
    </div>
  )
}
