import { useState, type FormEvent } from 'react'
import type { Address, AddressInput } from '@/shared/api/types'
import { Button } from '@/shared/ui/Button'
import { TextField } from '@/shared/ui/TextField'
import styles from './Addresses.module.css'

const empty: AddressInput = {
  label: '',
  full_name: '',
  phone_number: '',
  address_line1: '',
  address_line2: '',
  landmark: '',
  city: '',
  pincode: '',
  is_default: false,
}

export function toInput(address: Address): AddressInput {
  return {
    label: address.label,
    full_name: address.full_name,
    phone_number: address.phone_number,
    address_line1: address.address_line1,
    address_line2: address.address_line2,
    landmark: address.landmark,
    city: address.city,
    pincode: address.pincode,
    is_default: address.is_default,
  }
}

type Props = {
  initial?: AddressInput
  submitLabel: string
  busy: boolean
  error: string | null
  onSubmit: (input: AddressInput) => void
  onCancel?: () => void
}

export function AddressForm({ initial, submitLabel, busy, error, onSubmit, onCancel }: Props) {
  const [value, setValue] = useState<AddressInput>(initial ?? empty)
  const set = (field: keyof AddressInput) => (event: { target: { value: string } }) =>
    setValue((current) => ({ ...current, [field]: event.target.value }))

  const submit = (event: FormEvent) => {
    event.preventDefault()
    onSubmit({ ...value, pincode: value.pincode.trim() })
  }

  return (
    <form className={styles.form} onSubmit={submit} noValidate>
      <div className={styles.grid2}>
        <TextField
          label="Full name"
          autoComplete="name"
          value={value.full_name}
          onChange={set('full_name')}
          required
        />
        <TextField
          label="Phone"
          type="tel"
          inputMode="tel"
          autoComplete="tel"
          value={value.phone_number}
          onChange={set('phone_number')}
          required
        />
      </div>
      <TextField
        label="House, building, street"
        autoComplete="address-line1"
        value={value.address_line1}
        onChange={set('address_line1')}
        required
      />
      <TextField
        label="Area, colony (optional)"
        autoComplete="address-line2"
        value={value.address_line2}
        onChange={set('address_line2')}
      />
      <div className={styles.grid2}>
        <TextField
          label="Pincode"
          inputMode="numeric"
          autoComplete="postal-code"
          maxLength={6}
          value={value.pincode}
          onChange={set('pincode')}
          hint="City and state fill in from this"
          required
        />
        <TextField
          label="City (optional)"
          autoComplete="address-level2"
          value={value.city}
          onChange={set('city')}
        />
      </div>
      <div className={styles.grid2}>
        <TextField label="Landmark (optional)" value={value.landmark} onChange={set('landmark')} />
        <TextField
          label="Label (optional)"
          placeholder="Home, Work…"
          value={value.label}
          onChange={set('label')}
        />
      </div>
      <label className={styles.check}>
        <input
          type="checkbox"
          checked={value.is_default}
          onChange={(event) =>
            setValue((current) => ({ ...current, is_default: event.target.checked }))
          }
        />
        Make this my default address
      </label>
      {error && <p className={styles.error}>{error}</p>}
      <div className={styles.actions}>
        {onCancel && (
          <Button variant="ghost" onClick={onCancel} disabled={busy}>
            Cancel
          </Button>
        )}
        <Button type="submit" variant="accent" loading={busy}>
          {submitLabel}
        </Button>
      </div>
    </form>
  )
}

export function AddressLines({
  address,
}: {
  address: Pick<
    Address,
    | 'full_name'
    | 'phone_number'
    | 'address_line1'
    | 'address_line2'
    | 'landmark'
    | 'city'
    | 'state'
    | 'pincode'
  >
}) {
  return (
    <div className={styles.lines}>
      <strong>{address.full_name}</strong>
      <span>
        {[address.address_line1, address.address_line2, address.landmark]
          .filter(Boolean)
          .join(', ')}
      </span>
      <span>
        {address.city}, {address.state} {address.pincode}
      </span>
      <span className={styles.muted}>{address.phone_number}</span>
    </div>
  )
}
