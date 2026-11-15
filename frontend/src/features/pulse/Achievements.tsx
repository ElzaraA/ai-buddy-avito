import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { Achievement } from '../../api/schemas';
import { useChatStore } from '../../store/chat';
import { ACTIONS_REFRESH_MS } from '../progress/MyActions';

/** Иконка значка: звезда для полученного, замок для закрытого (декоративно — статус дублируется текстом). */
function BadgeIcon({ unlocked }: { unlocked: boolean }) {
  return unlocked ? (
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path
        d="M12 2.8l2.7 5.6 6.1.8-4.5 4.2 1.1 6-5.4-2.9-5.4 2.9 1.1-6-4.5-4.2 6.1-.8z"
        fill="#ffe45c"
        stroke="#1f1d2b"
        strokeWidth="1.3"
        strokeLinejoin="round"
      />
    </svg>
  ) : (
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
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

/**
 * «Достижения» на рабочем столе: награждают правильные привычки — спросить Бадди,
 * оформить заявку самому, отметить «Пульс» и не стесняться позвать наставника.
 */
export function Achievements() {
  const user = useChatStore((s) => s.user);
  const q = useQuery({ queryKey: ['achievements', user], queryFn: api.achievements, refetchInterval: ACTIONS_REFRESH_MS });
  const list = q.data ?? [];
  if (q.isError || list.length === 0) return null;

  const got = list.filter((a) => a.unlocked).length;
  const next = nextAchievement(list);

  return (
    <section className="achievements" aria-labelledby="achievements-title">
      <div className="achievements__head">
        <h2 id="achievements-title" className="achievements__title">
          Достижения
        </h2>
        <span className="achievements__count">
          получено {got} из {list.length}
        </span>
      </div>
      {next ? (
        <p className="achievements__next">
          <strong>Следующее: {next.title}.</strong> {next.hint}
        </p>
      ) : (
        <p className="achievements__next">
          <strong>Все достижения получены!</strong> Вы отлично проходите адаптацию.
        </p>
      )}
      <ul className="achievements__list">
        {list.map((a) => (
          <li
            key={a.id}
            className={`badge-card${a.unlocked ? ' badge-card--on' : ''}${a.id === next?.id ? ' badge-card--next' : ''}`}
            title={a.unlocked ? a.description : a.hint}
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
    </section>
  );
}
