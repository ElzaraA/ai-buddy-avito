/** Шкала «Пульса»: подписи и цвета. Лицо рисуется SVG, без эмодзи — одинаково на любой системе. */
export const MOODS = [
  { value: 1, label: 'Тяжело', color: '#f4b8ab' },
  { value: 2, label: 'Так себе', color: '#ffd3a8' },
  { value: 3, label: 'Нормально', color: '#fbe7a1' },
  { value: 4, label: 'Хорошо', color: '#d3ebbb' },
  { value: 5, label: 'Отлично', color: '#a9dcc5' },
] as const;

export const moodLabel = (m: number | null | undefined) => MOODS.find((x) => x.value === m)?.label ?? '';
export const moodColor = (m: number | null | undefined) => MOODS.find((x) => x.value === m)?.color ?? 'transparent';

/** Простое лицо: изгиб рта от грустного (1) к улыбке (5). Декоративное — подпись всегда текстом рядом. */
export function MoodFace({ mood, size = 28 }: { mood: number; size?: number }) {
  const curve = (mood - 3) * 2.2; // <0 — грусть, >0 — улыбка
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" focusable="false" className="mood-face">
      <circle cx="12" cy="12" r="11" fill={moodColor(mood)} stroke="rgba(31,29,43,.35)" strokeWidth="1" />
      <circle cx="8.5" cy="10" r="1.3" fill="#1f1d2b" />
      <circle cx="15.5" cy="10" r="1.3" fill="#1f1d2b" />
      <path
        d={`M7.5 ${15.5 - curve / 2} Q12 ${15.5 + curve} 16.5 ${15.5 - curve / 2}`}
        fill="none"
        stroke="#1f1d2b"
        strokeWidth="1.6"
        strokeLinecap="round"
      />
    </svg>
  );
}
