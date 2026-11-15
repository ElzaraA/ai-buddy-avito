import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { Achievement } from '../../api/schemas';
import { useChatStore } from '../../store/chat';
import { ACTIONS_REFRESH_MS } from '../progress/MyActions';

/** Иконка значка: звезда для полученного, замок для закрытого (декоративно — статус дублируется текстом). */
function BadgeIcon({ unlocked, size = 18 }: { unlocked: boolean; size?: number }) {
  return unlocked ? (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path
        d="M12 2.8l2.7 5.6 6.1.8-4.5 4.2 1.1 6-5.4-2.9-5.4 2.9 1.1-6-4.5-4.2 6.1-.8z"
        fill="#ffe45c"
        stroke="#1f1d2b"
        strokeWidth="1.3"
        strokeLinejoin="round"
      />
    </svg>
  ) : (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <rect x="5" y="10.5" width="14" height="10" rx="2.5" fill="none" stroke="#66636f" strokeWidth="1.6" />
      <path d="M8.5 10.5V8a3.5 3.5 0 0 1 7 0v2.5" fill="none" stroke="#66636f" strokeWidth="1.6" />
    </svg>
  );
}

const dateLabel = (iso?: string) => {
  if (!iso) return '';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' });
};

/** Следующее достижимое: первое неполученное в порядке от простых к сложным. */
export const nextAchievement = (list: Achievement[]) => list.find((a) => !a.unlocked);

/** Сколько полученных значков видно в свёрнутой строке. */
export const RECENT_LIMIT = 3;

/** Полученные — самые свежие первыми (при равном времени — в порядке сервера), не больше limit. */
export const recentUnlocked = (list: Achievement[], limit = RECENT_LIMIT) =>
  list
    .map((a, i) => ({ a, i }))
    .filter(({ a }) => a.unlocked)
    .sort((x, y) => (y.a.unlocked_at ?? '').localeCompare(x.a.unlocked_at ?? '') || x.i - y.i)
    .slice(0, limit)
    .map(({ a }) => a);

/**
 * «Достижения» на рабочем столе: награждают правильные привычки — спросить Бадди,
 * оформить заявку самому, отметить «Пульс» и не стесняться позвать наставника.
 * Свёрнуто — одна строка (свежие значки и следующее), чтобы не вытеснять чат; «Все достижения» раскрывает список.
 */
export function Achievements() {
  const user = useChatStore((s) => s.user);
  const [open, setOpen] = useState(false);
  const q = useQuery({ queryKey: ['achievements', user], queryFn: api.achievements, refetchInterval: ACTIONS_REFRESH_MS });
  const list = q.data ?? [];
  if (q.isError || list.length === 0) return null;

  const got = list.filter((a) => a.unlocked).length;
  const next = nextAchievement(list);
  const recent = recentUnlocked(list);

  return (
    <section className={`achievements${open ? ' achievements--open' : ''}`} aria-labelledby="achievements-title">
      <div className="achievements__bar">
        <h2 id="achievements-title" className="achievements__title">
          Достижения
        </h2>
        <span className="achievements__count">
          получено {got} из {list.length}
        </span>
        {recent.length > 0 && (
          <ul className="achievements__recent" aria-label="Последние полученные">
            {recent.map((a) => (
              <li key={a.id} className="ach-chip" title={a.description}>
                <BadgeIcon unlocked size={14} />
                {a.title}
              </li>
            ))}
          </ul>
        )}
        {next ? (
          <p className="achievements__next" title={next.hint}>
            <strong>Следующее: {next.title}.</strong> {next.hint}
          </p>
        ) : (
          <p className="achievements__next">
            <strong>Все достижения получены!</strong> Вы отлично проходите адаптацию.
          </p>
        )}
        <button
          type="button"
          className="achievements__toggle"
          aria-expanded={open}
          aria-controls="achievements-list"
          // видимое «Все» — начало доступного имени (WCAG 2.5.3), кнопка короткая и помещается в колонку
          aria-label={open ? 'Свернуть достижения' : 'Все достижения'}
          onClick={() => setOpen((v) => !v)}
        >
          {open ? 'Свернуть' : 'Все'}
          <span className="achievements__caret" aria-hidden="true">
            {open ? '▴' : '▾'}
          </span>
        </button>
      </div>
      {open && (
        <ul id="achievements-list" className="achievements__list" aria-label="Все достижения">
          {list.map((a) => (
            <li
              key={a.id}
              className={`badge-card${a.unlocked ? ' badge-card--on' : ''}${a.id === next?.id ? ' badge-card--next' : ''}`}
            >
              <BadgeIcon unlocked={a.unlocked} />
              <span className="badge-card__text">
                <span className="badge-card__title">{a.title}</span>
                <span className="badge-card__state">
                  {a.unlocked
                    ? `получено${a.unlocked_at ? ` ${dateLabel(a.unlocked_at)}` : ''}`
                    : a.id === next?.id
                      ? 'следующее'
                      : 'пока закрыто'}
                </span>
                <span className="badge-card__hint">{a.unlocked ? a.description : a.hint}</span>
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
