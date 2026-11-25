import { Mascot } from '@/shared/ui/doodles'

export function Logo({ size = 34 }: { size?: number }) {
  return <Mascot size={size} mood="happy" />
}
