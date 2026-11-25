import { Minus, Plus, Trash2 } from 'lucide-react'
import styles from './QuantityStepper.module.css'

type Props = {
  value: number
  max: number
  onChange: (value: number) => void
  disabled?: boolean
  label: string
}

export function QuantityStepper({ value, max, onChange, disabled, label }: Props) {
  return (
    <div className={styles.stepper} role="group" aria-label={`Quantity for ${label}`}>
      <button
        type="button"
        className={styles.step}
        onClick={() => onChange(value - 1)}
        disabled={disabled}
        aria-label={value <= 1 ? 'Remove' : 'Decrease quantity'}
      >
        {value <= 1 ? <Trash2 size={16} /> : <Minus size={16} />}
      </button>
      <span className={styles.value} aria-live="polite">
        {value}
      </span>
      <button
        type="button"
        className={styles.step}
        onClick={() => onChange(value + 1)}
        disabled={disabled || value >= max}
        aria-label="Increase quantity"
      >
        <Plus size={16} />
      </button>
    </div>
  )
}
