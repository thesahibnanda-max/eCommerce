import type { ReactNode } from 'react'
import { Heart, Mascot, Sparkle, Sticker, type MascotMood, type Tone } from './doodles'
import styles from './EmptyState.module.css'

type Props = {
  mood?: MascotMood
  tone?: Tone
  sign?: string
  title: string
  message?: string
  action?: ReactNode
}

export function EmptyState({ mood = 'happy', tone = 'pink', sign, title, message, action }: Props) {
  return (
    <div className={styles.empty}>
      <div className={styles.art}>
        <span className={styles.halo} aria-hidden />
        <Mascot size={120} mood={mood} tone={tone} sign={sign} className={styles.mascot} />
        <Sticker className={styles.s1} tilt={-14}>
          <Sparkle size={26} />
        </Sticker>
        <Sticker className={styles.s2} tilt={12} delay={1}>
          <Heart size={22} />
        </Sticker>
      </div>
      <h2 className={styles.title}>{title}</h2>
      {message && <p className={styles.message}>{message}</p>}
      {action}
    </div>
  )
}
