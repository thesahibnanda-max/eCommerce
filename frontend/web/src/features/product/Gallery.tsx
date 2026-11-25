import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useRef, useState } from 'react'
import { SmartImage } from '@/shared/ui/SmartImage'
import styles from './ProductPage.module.css'

export function Gallery({ images, alt }: { images: string[]; alt: string }) {
  const [index, setIndex] = useState(0)
  const track = useRef<HTMLDivElement>(null)
  const list = images.length > 0 ? images : [undefined]
  const many = list.length > 1
  const onScroll = () => {
    const node = track.current
    if (!node || node.clientWidth === 0) return
    setIndex(Math.round(node.scrollLeft / node.clientWidth))
  }
  const go = (i: number) => {
    const node = track.current
    if (!node) return
    const next = (i + list.length) % list.length
    node.scrollTo({ left: next * node.clientWidth, behavior: 'smooth' })
  }
  return (
    <div className={styles.gallery} data-many={many || undefined}>
      <div className={styles.main}>
        <div
          ref={track}
          className={styles.track}
          onScroll={onScroll}
          tabIndex={many ? 0 : undefined}
          aria-label={many ? `${alt} photos` : undefined}
          onKeyDown={(event) => {
            if (event.key === 'ArrowRight') go(index + 1)
            if (event.key === 'ArrowLeft') go(index - 1)
          }}
        >
          {list.map((src, i) => (
            <div key={src ?? i} className={styles.slide}>
              <SmartImage
                src={src}
                alt={`${alt} ${i + 1}`}
                priority={i === 0}
                ratio="1 / 1"
                fit="contain"
              />
            </div>
          ))}
        </div>
        {many && (
          <>
            <button
              type="button"
              className={`${styles.arrow} ${styles.prev}`}
              aria-label="Previous photo"
              onClick={() => go(index - 1)}
            >
              <ChevronLeft size={22} />
            </button>
            <button
              type="button"
              className={`${styles.arrow} ${styles.next}`}
              aria-label="Next photo"
              onClick={() => go(index + 1)}
            >
              <ChevronRight size={22} />
            </button>
            <div className={styles.dots} aria-hidden>
              {list.map((src, i) => (
                <span
                  key={src ?? i}
                  className={styles.dot}
                  data-active={i === index || undefined}
                />
              ))}
            </div>
          </>
        )}
      </div>
      {many && (
        <div className={styles.thumbs}>
          {list.map((src, i) => (
            <button
              key={src ?? i}
              type="button"
              className={styles.thumb}
              data-active={i === index || undefined}
              aria-label={`Show photo ${i + 1}`}
              aria-current={i === index || undefined}
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
