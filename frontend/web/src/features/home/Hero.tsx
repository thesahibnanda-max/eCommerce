import clsx from 'clsx'
import { Link } from 'react-router'
import { resolveImageURL } from '@/shared/api/config'
import { useInView } from '@/shared/hooks/useInView'
import { Button } from '@/shared/ui/Button'
import {
  Cloud,
  Flower,
  Heart,
  Mascot,
  Sparkle,
  Star,
  Sticker,
  toneAt,
  type Tone,
} from '@/shared/ui/doodles'
import styles from './Hero.module.css'

type Props = { images: (string | undefined)[]; onShop: () => void }

const polaroids: { className: string; caption: string; mood: 'happy' | 'wink' | 'love' }[] = [
  { className: styles.p1 ?? '', caption: 'new in ✿', mood: 'happy' },
  { className: styles.p2 ?? '', caption: 'so cute!', mood: 'love' },
  { className: styles.p3 ?? '', caption: 'made in India', mood: 'wink' },
]

function Polaroid({
  image,
  caption,
  className,
  tone,
  mood,
  priority,
}: {
  image: string | undefined
  caption: string
  className: string
  tone: Tone
  mood: 'happy' | 'wink' | 'love'
  priority: boolean
}) {
  const url = resolveImageURL(image)
  return (
    <figure className={clsx(styles.polaroid, styles[tone], className)}>
      <div className={styles.photo}>
        {url ? (
          <img
            src={url}
            alt=""
            loading={priority ? 'eager' : 'lazy'}
            decoding="async"
            fetchPriority={priority ? 'high' : 'auto'}
          />
        ) : (
          <Mascot size={70} mood={mood} tone={tone} />
        )}
      </div>
      <figcaption>{caption}</figcaption>
    </figure>
  )
}

export function Hero({ images, onShop }: Props) {
  const [artRef, visible] = useInView<HTMLDivElement>()
  return (
    <section className={styles.hero}>
      <div className={styles.copy}>
        <span className={styles.eyebrow}>
          <Sparkle size={16} /> fresh drops every week
        </span>
        <h1 className={styles.title}>
          Cute finds,
          <br />
          <span className={styles.highlight}>desi soul.</span>
        </h1>
        <p className={styles.subtitle}>
          Little pieces of India, handpicked with love and wrapped up just for you.
        </p>
        <div className={styles.actions}>
          <Button size="lg" onClick={onShop}>
            Shop new in
          </Button>
          <Link to="/search">
            <Button size="lg" variant="ghost">
              Explore all
            </Button>
          </Link>
        </div>
      </div>

      <div ref={artRef} className={clsx(styles.art, !visible && styles.paused)} aria-hidden>
        <span className={styles.blob} />
        {polaroids.map((item, index) => (
          <Polaroid
            key={item.caption}
            image={images[index]}
            caption={item.caption}
            className={item.className}
            tone={toneAt(index)}
            mood={item.mood}
            priority={index === 0}
          />
        ))}
        <Mascot size={112} mood="happy" wave className={styles.mascot} />
        <Sticker className={styles.s1} tilt={-12}>
          <Heart size={34} />
        </Sticker>
        <Sticker className={styles.s2} tilt={14} delay={0.6}>
          <Sparkle size={30} />
        </Sticker>
        <Sticker className={styles.s3} tilt={-6} delay={1.2}>
          <Flower size={34} />
        </Sticker>
        <Sticker className={styles.s4} tilt={8} delay={1.8}>
          <Star size={28} tone="peach" />
        </Sticker>
        <Sticker className={styles.s5} delay={0.3}>
          <Cloud size={54} />
        </Sticker>
      </div>
    </section>
  )
}
