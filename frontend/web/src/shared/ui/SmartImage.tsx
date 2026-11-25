import clsx from 'clsx'
import { useState } from 'react'
import { resolveImageURL } from '@/shared/api/config'
import { Mascot } from './doodles'
import styles from './SmartImage.module.css'

type Props = {
  src: string | undefined
  alt: string
  className?: string
  priority?: boolean
  ratio?: string
}

export function SmartImage({ src, alt, className, priority, ratio = '4 / 5' }: Props) {
  const url = resolveImageURL(src)
  const [state, setState] = useState<'loading' | 'loaded' | 'failed'>('loading')
  return (
    <div className={clsx(styles.frame, className)} style={{ aspectRatio: ratio }}>
      {url && state !== 'failed' ? (
        <img
          className={clsx(styles.image, state === 'loaded' && styles.loaded)}
          src={url}
          alt={alt}
          loading={priority ? 'eager' : 'lazy'}
          decoding="async"
          fetchPriority={priority ? 'high' : 'auto'}
          onLoad={() => setState('loaded')}
          onError={() => setState('failed')}
        />
      ) : (
        <div className={styles.placeholder} aria-hidden>
          <Mascot size={56} mood="sleepy" tone="lilac" />
        </div>
      )}
    </div>
  )
}
