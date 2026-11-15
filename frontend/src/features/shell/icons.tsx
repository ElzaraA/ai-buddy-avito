/** Простые линейные иконки 20×20: декоративные, поэтому скрыты от скринридеров. */
type IconProps = { className?: string };

const base = {
  width: 20,
  height: 20,
  viewBox: '0 0 20 20',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.6,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
  'aria-hidden': true,
  focusable: false,
};

export const IconChat = (p: IconProps) => (
  <svg {...base} {...p}>
    <path d="M4 4.5h12a1.5 1.5 0 0 1 1.5 1.5v7a1.5 1.5 0 0 1-1.5 1.5H9l-3.5 3v-3H4A1.5 1.5 0 0 1 2.5 13V6A1.5 1.5 0 0 1 4 4.5Z" />
    <path d="M6.5 8.5h7M6.5 11h4" />
  </svg>
);

export const IconPlan = (p: IconProps) => (
  <svg {...base} {...p}>
    <rect x="3.5" y="3" width="13" height="14.5" rx="2" />
    <path d="m6.5 7.5 1.2 1.2 2.3-2.4M11.5 8h2.5M6.5 12.5l1.2 1.2 2.3-2.4M11.5 13h2.5" />
  </svg>
);

export const IconInbox = (p: IconProps) => (
  <svg {...base} {...p}>
    <path d="M3 11.5 5 4.5h10l2 7v4a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 3 15.5v-4Z" />
    <path d="M3 11.5h4l1 2h4l1-2h4" />
  </svg>
);

export const IconSpark = (p: IconProps) => (
  <svg {...base} {...p}>
    <path d="M10 2.5v3M10 14.5v3M2.5 10h3M14.5 10h3" />
    <path d="M10 6.5 11.2 8.8 13.5 10l-2.3 1.2L10 13.5l-1.2-2.3L6.5 10l2.3-1.2L10 6.5Z" />
  </svg>
);

export const IconFlag = (p: IconProps) => (
  <svg {...base} {...p}>
    <path d="M4.5 17.5V3.5" />
    <path d="M4.5 4h10l-2 3.5 2 3.5h-10" />
  </svg>
);

export const IconTeam = (p: IconProps) => (
  <svg {...base} {...p}>
    <circle cx="7.5" cy="7" r="2.6" />
    <circle cx="14" cy="8" r="2.1" />
    <path d="M2.5 16.5c.6-2.8 2.6-4.3 5-4.3s4.4 1.5 5 4.3M12.6 12.4c2.3-.4 4.3.9 4.9 3.6" />
  </svg>
);

export const IconArrow = (p: IconProps) => (
  <svg {...base} {...p}>
    <path d="M4 10h12M11.5 5.5 16 10l-4.5 4.5" />
  </svg>
);

/** Аватар Бадди: дружелюбный робот-кружок, нарисован целиком в SVG. */
export function BuddyAvatar({ size = 56 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 56 56" aria-hidden focusable={false} className="avatar">
      <defs>
        <linearGradient id="buddy-bg" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#FFE7B8" />
          <stop offset="1" stopColor="#F9C9DF" />
        </linearGradient>
      </defs>
      <rect width="56" height="56" rx="18" fill="url(#buddy-bg)" />
      <path d="M28 9v6" stroke="#1F1D2B" strokeWidth="2" strokeLinecap="round" />
      <circle cx="28" cy="8" r="2.6" fill="#7FC4D8" stroke="#1F1D2B" strokeWidth="1.6" />
      <rect x="12" y="15" width="32" height="27" rx="12" fill="#DDEFC6" stroke="#1F1D2B" strokeWidth="2" />
      <circle cx="21.5" cy="27" r="3" fill="#1F1D2B" />
      <circle cx="34.5" cy="27" r="3" fill="#1F1D2B" />
      <circle cx="22.5" cy="26" r="0.9" fill="#fff" />
      <circle cx="35.5" cy="26" r="0.9" fill="#fff" />
      <path d="M23 33.5c2.8 2.6 7.2 2.6 10 0" stroke="#1F1D2B" strokeWidth="2" strokeLinecap="round" fill="none" />
      <circle cx="17" cy="32.5" r="2" fill="#F6A6C8" opacity=".8" />
      <circle cx="39" cy="32.5" r="2" fill="#F6A6C8" opacity=".8" />
      <path d="M17 42v4.5M39 42v4.5" stroke="#1F1D2B" strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}
