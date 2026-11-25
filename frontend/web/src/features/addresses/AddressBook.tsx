import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { MapPin, Plus } from 'lucide-react'
import { useState } from 'react'
import {
  createAddress,
  deleteAddress,
  listAddresses,
  setDefaultAddress,
  updateAddress,
} from '@/shared/api/addresses'
import { errorMessage } from '@/shared/api/errors'
import { queryKeys } from '@/shared/api/queries'
import type { Address, AddressInput } from '@/shared/api/types'
import { toast } from '@/shared/lib/toastStore'
import { Button } from '@/shared/ui/Button'
import { Skeleton } from '@/shared/ui/Skeleton'
import { AddressForm, AddressLines, toInput } from './AddressForm'
import styles from './Addresses.module.css'

export function useAddresses(enabled = true) {
  return useQuery({
    queryKey: queryKeys.addresses,
    queryFn: ({ signal }) => listAddresses(signal),
    enabled,
    staleTime: 60_000,
  })
}

export function useSaveAddress(onSaved?: (address: Address) => void) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id?: string; input: AddressInput }) =>
      id ? updateAddress(id, input) : createAddress(input),
    onSuccess: (address) => {
      void client.invalidateQueries({ queryKey: queryKeys.addresses })
      onSaved?.(address)
    },
  })
}

export function AddressBook() {
  const client = useQueryClient()
  const { data: addresses, isLoading } = useAddresses()
  const [editing, setEditing] = useState<string | 'new' | null>(null)
  const save = useSaveAddress(() => {
    setEditing(null)
    toast('Address saved', 'success')
  })
  const remove = useMutation({
    mutationFn: deleteAddress,
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.addresses }),
    onError: (error) => toast(errorMessage(error), 'error'),
  })
  const makeDefault = useMutation({
    mutationFn: setDefaultAddress,
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.addresses }),
    onError: (error) => toast(errorMessage(error), 'error'),
  })

  if (isLoading) return <Skeleton style={{ height: 120, borderRadius: 'var(--radius-lg)' }} />

  return (
    <div className={styles.form}>
      <ul className={styles.list}>
        {(addresses ?? []).map((address) => (
          <li key={address.id} className={styles.card}>
            {editing === address.id ? (
              <AddressForm
                initial={toInput(address)}
                submitLabel="Save"
                busy={save.isPending}
                error={save.error ? errorMessage(save.error) : null}
                onSubmit={(input) => save.mutate({ id: address.id, input })}
                onCancel={() => setEditing(null)}
              />
            ) : (
              <>
                <div className={styles.cardHead}>
                  <span className={styles.muted}>
                    <MapPin size={14} /> {address.label || 'Address'}
                  </span>
                  {address.is_default && <span className={styles.tag}>Default</span>}
                </div>
                <AddressLines address={address} />
                <div className={styles.cardActions}>
                  <button
                    type="button"
                    className={styles.linkButton}
                    onClick={() => setEditing(address.id)}
                  >
                    Edit
                  </button>
                  {!address.is_default && (
                    <button
                      type="button"
                      className={styles.linkButton}
                      disabled={makeDefault.isPending}
                      onClick={() => makeDefault.mutate(address.id)}
                    >
                      Make default
                    </button>
                  )}
                  <button
                    type="button"
                    className={`${styles.linkButton} ${styles.danger}`}
                    disabled={remove.isPending}
                    onClick={() => remove.mutate(address.id)}
                  >
                    Delete
                  </button>
                </div>
              </>
            )}
          </li>
        ))}
      </ul>
      {editing === 'new' ? (
        <div className={styles.card}>
          <AddressForm
            submitLabel="Add address"
            busy={save.isPending}
            error={save.error ? errorMessage(save.error) : null}
            onSubmit={(input) => save.mutate({ input })}
            onCancel={() => setEditing(null)}
          />
        </div>
      ) : (
        <Button variant="ghost" block icon={<Plus size={16} />} onClick={() => setEditing('new')}>
          Add a new address
        </Button>
      )}
    </div>
  )
}
