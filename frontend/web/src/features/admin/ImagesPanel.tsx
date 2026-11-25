import { Copy } from 'lucide-react'
import { useState } from 'react'
import {
  deleteImage,
  deleteImages,
  getImageInfo,
  uploadImage,
  uploadImages,
} from '@/shared/api/admin'
import type { ImageInfo } from '@/shared/api/types'
import { Button } from '@/shared/ui/Button'
import { SmartImage } from '@/shared/ui/SmartImage'
import { TextArea, TextField } from '@/shared/ui/TextField'
import type { AdminContext } from './AdminPage'
import { parseJSON, splitList } from './parse'
import styles from './AdminPage.module.css'

function isImageInfo(value: unknown): value is ImageInfo {
  return !!value && typeof value === 'object' && typeof (value as ImageInfo).id === 'string'
}

export function ImagesPanel({ context }: { context: AdminContext }) {
  const { run, busy } = context
  const [file, setFile] = useState<File | null>(null)
  const [metadata, setMetadata] = useState('')
  const [files, setFiles] = useState<File[]>([])
  const [uploaded, setUploaded] = useState<ImageInfo[]>([])
  const [lookupID, setLookupID] = useState('')
  const [deleteIDs, setDeleteIDs] = useState('')
  const [confirmDelete, setConfirmDelete] = useState(false)

  const remember = (result: unknown) => {
    const list = Array.isArray(result)
      ? result.filter(isImageInfo)
      : isImageInfo(result)
        ? [result]
        : []
    if (list.length > 0) setUploaded((items) => [...list, ...items].slice(0, 20))
  }

  const removeIDs = async () => {
    const ids = splitList(deleteIDs)
    if (ids.length === 0) return
    if (!confirmDelete) {
      setConfirmDelete(true)
      return
    }
    setConfirmDelete(false)
    const result = await run(`Delete ${ids.length} image(s)`, (key) =>
      ids.length === 1 ? deleteImage(key, ids[0] ?? '') : deleteImages(key, ids),
    )
    if (result) {
      setDeleteIDs('')
      setUploaded((items) => items.filter((item) => !ids.includes(item.id)))
    }
  }

  return (
    <div className={styles.panel}>
      <div className={styles.card}>
        <h2 className={styles.sectionTitle}>Upload one image</h2>
        <input
          className={styles.file}
          type="file"
          accept="image/*"
          onChange={(e) => setFile(e.target.files?.[0] ?? null)}
        />
        <TextArea
          label="Metadata (optional JSON object)"
          value={metadata}
          onChange={(e) => setMetadata(e.target.value)}
          style={{ minHeight: 80 }}
        />
        <Button
          variant="accent"
          loading={busy}
          disabled={!file}
          onClick={async () => {
            if (!file) return
            const result = await run(`Upload ${file.name}`, (key) => {
              const parsed = metadata.trim() ? parseJSON(metadata) : null
              if (parsed !== null && (typeof parsed !== 'object' || Array.isArray(parsed)))
                throw new Error('Metadata must be a JSON object')
              return uploadImage(key, file, parsed as Record<string, unknown> | null)
            })
            remember(result)
          }}
        >
          Upload
        </Button>
      </div>

      <div className={styles.card}>
        <h2 className={styles.sectionTitle}>Upload many</h2>
        <input
          className={styles.file}
          type="file"
          accept="image/*"
          multiple
          onChange={(e) => setFiles([...(e.target.files ?? [])])}
        />
        <Button
          variant="accent"
          loading={busy}
          disabled={files.length === 0}
          onClick={async () =>
            remember(await run(`Upload ${files.length} images`, (key) => uploadImages(key, files)))
          }
        >
          Upload {files.length > 0 ? files.length : ''} images
        </Button>
      </div>

      {uploaded.length > 0 && (
        <div className={styles.card}>
          <h2 className={styles.sectionTitle}>Recent uploads</h2>
          <div className={styles.thumbs}>
            {uploaded.map((image) => (
              <div key={image.id} className={styles.thumb}>
                <SmartImage src={image.id} alt={image.file_name} ratio="1 / 1" />
                <button
                  type="button"
                  className={styles.copy}
                  onClick={() => void navigator.clipboard?.writeText(image.id)}
                  aria-label={`Copy id of ${image.file_name}`}
                >
                  <Copy size={14} /> {image.id.slice(-6)}
                </button>
                <span className={styles.thumbMeta}>
                  {image.width}×{image.height}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className={styles.card}>
        <h2 className={styles.sectionTitle}>Look up</h2>
        <div className={styles.inline}>
          <TextField
            label="Image ID"
            value={lookupID}
            onChange={(e) => setLookupID(e.target.value)}
          />
          <Button
            variant="ghost"
            disabled={!lookupID.trim()}
            onClick={async () =>
              remember(await run(`Info ${lookupID.trim()}`, () => getImageInfo(lookupID.trim())))
            }
          >
            Fetch
          </Button>
        </div>
      </div>

      <div className={styles.card}>
        <h2 className={styles.sectionTitle}>Delete</h2>
        <TextField
          label="Image IDs"
          hint="Comma separated. Deleting cannot be undone."
          value={deleteIDs}
          onChange={(e) => {
            setDeleteIDs(e.target.value)
            setConfirmDelete(false)
          }}
        />
        <Button
          variant="danger"
          loading={busy}
          disabled={!deleteIDs.trim()}
          onClick={() => void removeIDs()}
        >
          {confirmDelete ? 'Tap again to delete permanently' : 'Delete'}
        </Button>
      </div>
    </div>
  )
}
