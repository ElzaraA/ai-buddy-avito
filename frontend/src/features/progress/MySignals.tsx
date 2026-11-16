import { useQuery } from '@tanstack/react-query';
import { api, ApiError } from '../../api/client';
import type { Signal } from '../../api/schemas';
import { useChatStore } from '../../store/chat';
import { ACTIONS_REFRESH_MS } from './MyActions';

/** 404 — у бэкенда ещё нет ручки /signals/me: не повторяем и не опрашиваем, блок просто не показывается. */
const isMissing = (e: unknown) => e instanceof ApiError && e.status === 404;

/** Обращения стажёра к наставнику: перечитываем так же часто, как заявки — ответ появляется сам. */
export function useMySignals() {
  const user = useChatStore((s) => s.user);
  return useQuery({
    queryKey: ['my-signals', user],
    queryFn: api.mySignals,
    retry: (count, e) => !isMissing(e) && count < 1,
    refetchInterval: (q) => (isMissing(q.state.error) ? false : ACTIONS_REFRESH_MS),
  });
}

export type SignalState = 'waiting' | 'answered' | 'closed';

/** ждёт ответа → наставник ответил (есть текст ответа) / разобрано (закрыто без ответа). */
export function signalState(s: Signal): SignalState {
  if (s.reply?.trim()) return 'answered';
  return s.done ? 'closed' : 'waiting';
}

export const SIGNAL_STATE_LABELS: Record<SignalState, string> = {
  waiting: 'ждёт ответа',
  answered: 'наставник ответил',
  closed: 'наставник разобрал',
};

const PREFIX = /^Передай наставнику:\s*/i;
const signalText = (s: Signal) => s.text.replace(PREFIX, '') || s.text;

function timeLabel(iso?: string) {
  if (!iso) return '';
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? ''
    : d.toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

/** Блок «Мои обращения к наставнику» в разделе «Мои заявки». */
export function MySignals() {
  const { data } = useMySignals();
  const list = data ?? [];
  if (list.length === 0) return null;

  return (
    <section className="block" aria-label="Мои обращения к наставнику">
      <h3 className="block__title">Мои обращения к наставнику</h3>
      <p className="block__note">То, что Бадди передал наставнику. Ответ появится здесь сам.</p>
      <ul className="signals">
        {list.map((s) => {
          const state = signalState(s);
          return (
            <li key={s.id} className={`signal signal--mine signal--${state}`}>
              <div className="signal__head">
                <span className={`badge badge--signal-${state}`}>{SIGNAL_STATE_LABELS[state]}</span>
                {s.at && <span className="signal__at">{timeLabel(s.at)}</span>}
              </div>
              <p className="signal__text">{signalText(s)}</p>
              {state === 'answered' && (
                <p className="signal__reply">
                  <span className="signal__reply-label">Ответ наставника:</span> {s.reply}
                </p>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}

/**
 * Компактный индикатор для карточки «Нужна помощь?» на рабочем столе:
 * сколько обращений ждёт ответа и последний ответ наставника.
 */
export function signalSummary(list: Signal[]) {
  const waiting = list.filter((s) => signalState(s) === 'waiting').length;
  const stamp = (s: Signal) => s.resolved_at ?? s.at ?? '';
  // самый свежий ответ (ISO-время сравнивается как строка)
  const answered = list
    .filter((s) => signalState(s) === 'answered')
    .reduce<Signal | undefined>((best, s) => (!best || stamp(s) > stamp(best) ? s : best), undefined);
  const parts = [
    waiting > 0 ? `ждёт ответа: ${waiting}` : '',
    answered ? `наставник ответил: «${answered.reply}»` : '',
  ].filter(Boolean);
  return { waiting, answered, text: parts.join('; ') };
}

export function SignalIndicator({ signals }: { signals: Signal[] }) {
  const { waiting, answered } = signalSummary(signals);
  if (!waiting && !answered) return null;
  return (
    <span className="desk-card__signals">
      {waiting > 0 && <span className="desk-card__signal">ждёт ответа: {waiting}</span>}
      {answered && (
        <span className="desk-card__signal desk-card__signal--answered">
          Наставник ответил: «{answered.reply}»
        </span>
      )}
    </span>
  );
}
