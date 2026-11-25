import { Link } from 'react-router'
import { infoLinks } from '@/shared/api/info'
import { Flower, Heart, Mascot, Sparkle, Sticker } from '@/shared/ui/doodles'
import styles from './Footer.module.css'

export function Footer() {
  return (
    <footer className={styles.footer}>
      <svg className={styles.wave} viewBox="0 0 1440 40" preserveAspectRatio="none" aria-hidden>
        <path d="M0 40V20c120-20 240-20 360 0s240 20 360 0 240-20 360 0 240 20 360 0v20z" />
      </svg>
      <div className={styles.inner}>
        <div className={styles.brand}>
          <Mascot size={64} mood="wink" />
          <div>
            <p className={styles.name}>Aesthetic Indians</p>
            <p className={styles.tagline}>handpicked with love, packed with care ♡</p>
          </div>
        </div>
        <div className={styles.columns}>
          <nav className={styles.links} aria-label="Shop">
            <Link to="/search">Shop all</Link>
            <Link to="/cart">Your bag</Link>
            <Link to="/account">Account</Link>
          </nav>
          <nav className={styles.policies} aria-label="Policies">
            {infoLinks.map((link) => (
              <Link key={link.slug} to={`/info/${link.slug}`}>
                {link.label}
              </Link>
            ))}
          </nav>
        </div>
        <p className={styles.small}>
          Made in India with chai and good vibes · {new Date().getFullYear()}
        </p>
        <Sticker className={styles.s1} tilt={-12}>
          <Heart size={26} />
        </Sticker>
        <Sticker className={styles.s2} tilt={10} delay={0.8}>
          <Sparkle size={24} />
        </Sticker>
        <Sticker className={styles.s3} tilt={6} delay={1.4}>
          <Flower size={26} tone="mint" />
        </Sticker>
      </div>
    </footer>
  )
}
