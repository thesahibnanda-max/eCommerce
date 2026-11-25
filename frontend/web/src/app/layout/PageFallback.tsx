import styles from './PageFallback.module.css'

export function PageFallback() {
  return (
    <div className={styles.wrap} aria-busy>
      <span className={styles.spinner} />
    </div>
  )
}
