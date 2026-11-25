import { useRef, useState } from 'react'
import { SmartImage } from '@/shared/ui/SmartImage'
import styles from './ProductPage.module.css'

export function Gallery({ images, alt }: { images: string[]; alt: string }) {
  const [index, setIndex] = useState(0)
  const track = useRef<HTMLDivElement>(null)
  const list = images.length > 0 ? images : [undefined]
  const onScroll = () => {
    const node = track.current
    if (!node || node.clientWidth === 0) return
    setIndex(Math.round(node.scrollLeft / node.clientWidth))
  }
  const go = (i: number) => {
    const node = track.current
    if (!node) return
    node.scrollTo({ left: i * node.clientWidth, behavior: 'smooth' })
  }
  return (
    <div className={styles.gallery}>
      <div ref={track} className={styles.track} onScroll={onScroll}>
        {list.map((src, i) => (
          <div key={src ?? i} className={styles.slide}>
            <SmartImage src={src} alt={`${alt} ${i + 1}`} priority={i === 0} />
          </div>
        ))}
      </div>
      {list.length > 1 && (
        <div className={styles.dots} aria-hidden>
          {list.map((src, i) => (
            <span key={src ?? i} className={styles.dot} data-active={i === index || undefined} />
          ))}
        </div>
      )}
      {list.length > 1 && (
        <div className={styles.thumbs}>
          {list.map((src, i) => (
            <button
              key={src ?? i}
              type="button"
              className={styles.thumb}
              data-active={i === index || undefined}
              aria-label={`Show image ${i + 1}`}
              onClick={() => go(i)}
            >
              <SmartImage src={src} alt="" ratio="1 / 1" />
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
