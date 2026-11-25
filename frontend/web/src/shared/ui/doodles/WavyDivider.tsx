import { useId } from 'react'
import { tones, type Tone } from './palette'

export function WavyDivider({ tone = 'pink', className }: { tone?: Tone; className?: string }) {
  const id = useId()
  return (
    <svg width="100%" height="14" className={className} aria-hidden>
      <defs>
        <pattern id={id} width="28" height="14" patternUnits="userSpaceOnUse">
          <path
            d="M0 7q7-7 14 0t14 0"
            stroke={tones[tone].shade}
            strokeWidth="3"
            strokeLinecap="round"
            fill="none"
          />
        </pattern>
      </defs>
      <rect width="100%" height="14" fill={`url(#${id})`} />
    </svg>
  )
}
