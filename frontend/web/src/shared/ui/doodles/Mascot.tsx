import type { CSSProperties } from 'react'
import { ink, tones, type Tone } from './palette'

export type MascotMood = 'happy' | 'wink' | 'sleepy' | 'love' | 'surprised'

type Props = {
  size?: number
  mood?: MascotMood
  tone?: Tone
  wave?: boolean
  sign?: string
  className?: string
  style?: CSSProperties
}

function Eyes({ mood }: { mood: MascotMood }) {
  const stroke = { stroke: ink, strokeWidth: 3.4, strokeLinecap: 'round' as const, fill: 'none' }
  switch (mood) {
    case 'sleepy':
      return (
        <>
          <path d="M42 64q6 5 12 0" {...stroke} />
          <path d="M66 64q6 5 12 0" {...stroke} />
        </>
      )
    case 'wink':
      return (
        <>
          <circle cx="48" cy="63" r="5" fill={ink} />
          <circle cx="49.8" cy="61.2" r="1.6" fill="#fff" />
          <path d="M66 64q6-6 12 0" {...stroke} />
        </>
      )
    case 'love':
      return (
        <>
          <path
            d="M48 69c-6-4-9-7-9-10.5a4.5 4.5 0 0 1 9-1 4.5 4.5 0 0 1 9 1C57 62 54 65 48 69z"
            fill="#ff4f86"
          />
          <path
            d="M72 69c-6-4-9-7-9-10.5a4.5 4.5 0 0 1 9-1 4.5 4.5 0 0 1 9 1C81 62 78 65 72 69z"
            fill="#ff4f86"
          />
        </>
      )
    case 'surprised':
      return (
        <>
          <circle cx="48" cy="62" r="6" fill={ink} />
          <circle cx="72" cy="62" r="6" fill={ink} />
          <circle cx="50" cy="60" r="2" fill="#fff" />
          <circle cx="74" cy="60" r="2" fill="#fff" />
        </>
      )
    default:
      return (
        <>
          <circle cx="48" cy="63" r="5" fill={ink} />
          <circle cx="72" cy="63" r="5" fill={ink} />
          <circle cx="49.8" cy="61.2" r="1.6" fill="#fff" />
          <circle cx="73.8" cy="61.2" r="1.6" fill="#fff" />
        </>
      )
  }
}

function Mouth({ mood }: { mood: MascotMood }) {
  if (mood === 'surprised') {
    return <ellipse cx="60" cy="78" rx="4" ry="5" fill={ink} />
  }
  if (mood === 'sleepy') {
    return (
      <path d="M56 78q4 3 8 0" stroke={ink} strokeWidth="3" strokeLinecap="round" fill="none" />
    )
  }
  return (
    <path
      d="M53 75q7 8 14 0"
      stroke={ink}
      strokeWidth="3.2"
      strokeLinecap="round"
      strokeLinejoin="round"
      fill="#ff7aa2"
    />
  )
}

export function Mascot({
  size = 96,
  mood = 'happy',
  tone = 'pink',
  wave,
  sign,
  className,
  style,
}: Props) {
  const color = tones[tone]
  const height = sign ? size * 1.35 : size
  return (
    <svg
      viewBox={sign ? '0 0 120 162' : '0 0 120 120'}
      width={size}
      height={height}
      className={className}
      style={style}
      aria-hidden
    >
      <ellipse cx="60" cy="112" rx="34" ry="5" fill={ink} opacity=".08" />
      <path
        d="M44 40c0-14 7-22 16-22s16 8 16 22"
        stroke={ink}
        strokeWidth="5.5"
        strokeLinecap="round"
        fill="none"
      />
      {wave && (
        <path
          d="M98 70c6-4 10-12 9-20"
          stroke={ink}
          strokeWidth="5"
          strokeLinecap="round"
          fill="none"
        />
      )}
      <path
        d="M26 46c0-6 4-10 10-10h48c6 0 10 4 10 10l4 50c.5 7-4 12-11 12H33c-7 0-11.5-5-11-12z"
        fill={color.fill}
        stroke={ink}
        strokeWidth="3.4"
        strokeLinejoin="round"
      />
      <path d="M30 46c0-3 2-5 5-5h50c3 0 5 2 5 5v4H30z" fill={color.light} opacity=".85" />
      <path
        d="M90 52l3 42c.4 5-2 8-6 9"
        stroke={color.shade}
        strokeWidth="4"
        strokeLinecap="round"
        fill="none"
      />
      <ellipse cx="38" cy="74" rx="6.5" ry="4" fill="#ff6f9a" opacity=".45" />
      <ellipse cx="82" cy="74" rx="6.5" ry="4" fill="#ff6f9a" opacity=".45" />
      <Eyes mood={mood} />
      <Mouth mood={mood} />
      {sign && (
        <g>
          <path d="M60 108v14" stroke={ink} strokeWidth="4" strokeLinecap="round" />
          <rect
            x="14"
            y="120"
            width="92"
            height="34"
            rx="10"
            fill="#fff"
            stroke={ink}
            strokeWidth="3"
          />
          <text
            x="60"
            y="142"
            textAnchor="middle"
            fontFamily="'Fredoka Variable', ui-rounded, sans-serif"
            fontWeight="600"
            fontSize="14"
            fill={ink}
          >
            {sign}
          </text>
        </g>
      )}
    </svg>
  )
}
