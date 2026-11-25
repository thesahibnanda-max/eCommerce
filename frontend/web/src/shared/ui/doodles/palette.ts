export const ink = '#3a2a3f'

export const tones = {
  pink: { fill: '#ff9cbb', shade: '#f27aa0', light: '#ffd6e3' },
  lilac: { fill: '#c3b1ff', shade: '#a28cf2', light: '#e7e0ff' },
  mint: { fill: '#a6ead0', shade: '#7ed4b3', light: '#dbf6eb' },
  butter: { fill: '#ffda7a', shade: '#f5c04a', light: '#fff1c7' },
  peach: { fill: '#ffbea3', shade: '#f79d7c', light: '#ffe4d8' },
  sky: { fill: '#a9d8ff', shade: '#7cbcf2', light: '#dcefff' },
} as const

export type Tone = keyof typeof tones

export const toneCycle: readonly Tone[] = ['pink', 'lilac', 'mint', 'butter', 'peach', 'sky']

export function toneAt(index: number): Tone {
  return toneCycle[((index % toneCycle.length) + toneCycle.length) % toneCycle.length] ?? 'pink'
}
