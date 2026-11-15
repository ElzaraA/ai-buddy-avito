import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { InternPulse } from '../../api/schemas';
import { useChatStore } from '../../store/chat';
import { MoodFace, moodColor, moodLabel } from '../pulse/mood';

const TREND_LABELS: Record<InternPulse['trend'], string> = {
  up: 'настроение растёт',
  down: 'настроение снижается',
  flat: 'без изменений',
};

const shortDate = (iso: string) => {
  const d = new Date(`${iso}T00:00:00`);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' });
};

const avgLabel = (v: number) => v.toFixed(1).replace('.', ',');

/** Сначала стажёры, которым нужно внимание (сервер уже сортирует — дублируем на случай старого бэка). */
export const sortByRisk = (list: InternPulse[]) => [...list].sort((a, b) => Number(b.risk) - Number(a.risk));

/**
 * «Пульс команды» в кабинете наставника: самочувствие стажёров за неделю и кому нужно внимание.
 * Необязательный блок: если сервер его не отдаёт, кабинет работает без него.
 */
export function TeamPulse({ refetchInterval }: { refetchInterval?: number }) {
  const user = useChatStore((s) => s.user);
  const q = useQuery({ queryKey: ['team-pulse', user], queryFn: api.teamPulse, refetchInterval });
  if (!q.data) return null;
  const list = sortByRisk(q.data);
  const atRisk = list.filter((p) => p.risk).length;

  return (
    <section className="block" aria-labelledby="team-pulse-title">
      <h3 id="team-pulse-title" className="block__title">
        Пульс команды
        {atRisk > 0 && (
          <span className="badge badge--risk signals__count" aria-label={`нужно внимание: ${atRisk}`}>
            {atRisk}
          </span>
        )}
      </h3>
      <p className="block__note">
        Как стажёры отмечают самочувствие за 7 дней. Заметки стажёров видны только наставнику и руководителю команды.
      </p>
      {list.length === 0 ? (
        <p className="block__note">В команде пока нет стажёров.</p>
      ) : (
        <ul className="pulse-team">
          {list.map((p) => (
            <li key={p.user.id} className={`pulse-row${p.risk ? ' pulse-row--risk' : ''}`}>
              <div className="pulse-row__head">
                <span className="pulse-row__name">{p.user.name}</span>
                <span className="pulse-row__today">
                  {p.today !== null ? (
                    <>
                      <MoodFace mood={p.today} size={18} />
                      сегодня: {moodLabel(p.today)}
                    </>
                  ) : (
                    'сегодня без отметки'
                  )}
                </span>
                {p.risk && <span className="badge badge--risk">нужно внимание</span>}
              </div>
              <div
                className="pulse-dots"
                role="img"
                aria-label={`Последние 7 дней: ${p.days.map((d) => (d.mood !== null ? moodLabel(d.mood) : 'нет отметки')).join(', ')}`}
              >
                {p.days.map((d) => (
                  <span
                    key={d.date}
                    className={`pulse-dot${d.mood === null ? ' pulse-dot--empty' : ''}`}
                    style={d.mood !== null ? { background: moodColor(d.mood) } : undefined}
                    title={`${shortDate(d.date)}: ${d.mood !== null ? moodLabel(d.mood) : 'нет отметки'}`}
                  />
                ))}
              </div>
              <p className="pulse-row__meta">
                {p.avg7 !== null ? `среднее за 7 дней: ${avgLabel(p.avg7)} из 5 · ${TREND_LABELS[p.trend]}` : 'за неделю отметок нет'}
              </p>
              {p.risk && p.risk_reason && <p className="pulse-row__reason">Почему: {p.risk_reason}</p>}
              {p.note && (
                <p className="pulse-row__note">
                  Заметка{p.note_date ? ` (${shortDate(p.note_date)})` : ''}: «{p.note}»
                </p>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
