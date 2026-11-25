import type { CSSProperties } from 'react'
import { ink, tones, type Tone } from './palette'

type ShapeProps = { size?: number; tone?: Tone; className?: string; style?: CSSProperties }

function frame(size: number, className?: string, style?: CSSProperties) {
  return { viewBox: '0 0 48 48', width: size, height: size, className, style, 'aria-hidden': true }
}

const outline = { stroke: ink, strokeWidth: 2.6, strokeLinejoin: 'round' as const }

export function Sparkle({ size = 28, tone = 'butter', className, style }: ShapeProps) {
  return (
    <svg {...frame(size, className, style)}>
      <path
        d="M24 4c2 10 6 14 20 20-14 6-18 10-20 20-2-10-6-14-20-20 14-6 18-10 20-20z"
        fill={tones[tone].fill}
        {...outline}
      />
    </svg>
  )
}

export function Heart({ size = 28, tone = 'pink', className, style }: ShapeProps) {
  return (
    <svg {...frame(size, className, style)}>
      <path
        d="M24 42C10 33 5 26 5 18a9.5 9.5 0 0 1 19-3 9.5 9.5 0 0 1 19 3c0 8-5 15-19 24z"
        fill={tones[tone].fill}
        {...outline}
      />
      <path
        d="M12 17a5 5 0 0 1 5-4"
        stroke="#fff"
        strokeWidth="2.6"
        strokeLinecap="round"
        fill="none"
      />
    </svg>
  )
}

export function Flower({ size = 30, tone = 'lilac', className, style }: ShapeProps) {
  const petals = [0, 72, 144, 216, 288]
  return (
    <svg {...frame(size, className, style)}>
      {petals.map((angle) => (
        <ellipse
          key={angle}
          cx="24"
          cy="12.5"
          rx="7.5"
          ry="9.5"
          transform={`rotate(${angle} 24 24)`}
          fill={tones[tone].fill}
          {...outline}
        />
      ))}
      <circle cx="24" cy="24" r="6.5" fill={tones.butter.fill} {...outline} />
    </svg>
  )
}

export function Star({ size = 30, tone = 'butter', className, style }: ShapeProps) {
  return (
    <svg {...frame(size, className, style)}>
      <path
        d="M24 5l5.6 11.6 12.7 1.8-9.2 8.9 2.2 12.6L24 34l-11.3 5.9 2.2-12.6-9.2-8.9 12.7-1.8z"
        fill={tones[tone].fill}
        {...outline}
      />
    </svg>
  )
}

export function Cloud({ size = 40, tone = 'sky', className, style }: ShapeProps) {
  return (
    <svg {...frame(size, className, style)}>
      <path
        d="M13 36a8 8 0 0 1-1-16 11 11 0 0 1 21-3 8.5 8.5 0 0 1 3 19z"
        fill={tones[tone].light}
        {...outline}
      />
    </svg>
  )
}

export function Swirl({ size = 30, tone = 'mint', className, style }: ShapeProps) {
  return (
    <svg {...frame(size, className, style)}>
      <path
        d="M24 24c0-2 2-3 3.5-2s1.5 4-1 5.5-6.5 0-7.5-3.5 1.5-8.5 6-9.5 10.5 2 11.5 7.5-3 12-9.5 13"
        stroke={tones[tone].shade}
        strokeWidth="4"
        strokeLinecap="round"
        fill="none"
      />
    </svg>
  )
}
