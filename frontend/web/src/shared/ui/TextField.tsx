import { useId, type InputHTMLAttributes, type TextareaHTMLAttributes } from 'react'
import styles from './TextField.module.css'

type Shared = { label: string; hint?: string; error?: string | null }

export function TextField({
  label,
  hint,
  error,
  id,
  ...rest
}: Shared & InputHTMLAttributes<HTMLInputElement>) {
  const generated = useId()
  const inputID = id ?? generated
  return (
    <label className={styles.field} htmlFor={inputID}>
      <span className={styles.label}>{label}</span>
      <input
        id={inputID}
        className={styles.input}
        aria-invalid={error ? true : undefined}
        {...rest}
      />
      {error ? (
        <span className={styles.error}>{error}</span>
      ) : hint ? (
        <span className={styles.hint}>{hint}</span>
      ) : null}
    </label>
  )
}

export function TextArea({
  label,
  hint,
  error,
  id,
  ...rest
}: Shared & TextareaHTMLAttributes<HTMLTextAreaElement>) {
  const generated = useId()
  const inputID = id ?? generated
  return (
    <label className={styles.field} htmlFor={inputID}>
      <span className={styles.label}>{label}</span>
      <textarea
        id={inputID}
        className={styles.input}
        aria-invalid={error ? true : undefined}
        {...rest}
      />
      {error ? (
        <span className={styles.error}>{error}</span>
      ) : hint ? (
        <span className={styles.hint}>{hint}</span>
      ) : null}
    </label>
  )
}
