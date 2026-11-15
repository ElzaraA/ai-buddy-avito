import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, ApiError } from '../../api/client';
import type { ChecklistItem } from '../../api/schemas';
import { useChatStore } from '../../store/chat';
import { actionTitle } from '../progress/MyActions';

/** Как часто перечитываем кабинет: новые заявки и вопросы стажёров появляются сами. */
export const TEAM_REFRESH_MS = 5000;

interface Props {
  /** buddy — наставник (только просмотр), lead — руководитель (согласует заявки) */
  role: string;
}

function segmentClass(it: ChecklistItem) {
  if (it.done) return 'track__seg track__seg--done';
  if (it.status === 'waiting') return 'track__seg track__seg--waiting';
  return 'track__seg';
}

/**
 * Кабинет наставника и руководителя: заявки на согласование, прогресс стажёров команды
 * и вопросы, на которые помощник не нашёл ответа в базе знаний.
 */
const TOPIC_LABELS: Record<string, string> = {
  tech: 'не работает',
  blocked: 'застрял',
  unclear: 'непонятно',
  other: 'вопрос',
};

function timeLabel(iso: string) {
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? ''
    : d.toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

export function TeamPanel({ role }: Props) {
  const user = useChatStore((s) => s.user);
  const qc = useQueryClient();
  const canDecide = role === 'lead';
  // комментарий у каждой заявки свой: видно, к какой заявке он уйдёт
  const [comments, setComments] = useState<Record<string, string>>({});
  const setComment = (id: string, text: string) => setComments((c) => ({ ...c, [id]: text }));

  const opts = { refetchInterval: TEAM_REFRESH_MS };
  const pending = useQuery({ queryKey: ['team-pending', user], queryFn: api.teamPending, ...opts });
  const progress = useQuery({ queryKey: ['team-progress', user], queryFn: api.teamProgress, ...opts });
  const gaps = useQuery({ queryKey: ['team-gaps', user], queryFn: api.knowledgeGaps, ...opts });
  // «метрики пользы»: необязательны — если сводка не загрузилась, кабинет работает без неё
  const summary = useQuery({ queryKey: ['team-summary', user], queryFn: api.teamSummary, ...opts });
  // сообщения стажёров «что-то не так»: необязательный блок, кабинет работает и без него
  const signals = useQuery({ queryKey: ['team-signals', user], queryFn: api.teamSignals, ...opts });
  const resolve = useMutation({
    mutationFn: api.resolveSignal,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['team-signals', user] }),
  });

  const refresh = () => {
    qc.invalidateQueries({ queryKey: ['team-pending', user] });
    qc.invalidateQueries({ queryKey: ['team-progress', user] });
    qc.invalidateQueries({ queryKey: ['team-summary', user] });
  };
  const decide = useMutation({
    mutationFn: (v: { id: string; decision: 'approve' | 'reject'; comment: string }) =>
      api.decideAction(v.id, v.decision, v.comment.trim()),
    onSuccess: (_data, v) => {
      setComments(({ [v.id]: _, ...rest }) => rest);
      refresh();
    },
    // 409: заявку уже отменили или решили — перечитываем список, чтобы она исчезла
    onError: (e) => {
      if (e instanceof ApiError && e.status === 409) refresh();
    },
  });

  if (pending.isError || progress.isError)
    return (
      <aside className="sheet sheet--state">
        <div className="error" role="alert">
          Не удалось загрузить данные команды.{' '}
          <button className="btn btn--link" onClick={() => void Promise.all([pending.refetch(), progress.refetch()])}>
            Повторить
          </button>
        </div>
      </aside>
    );
  if (pending.isLoading || progress.isLoading)
    return <aside className="sheet sheet--state">Загружаю данные команды…</aside>;

  const interns = progress.data ?? [];
  const nameOf = (id?: string) => interns.find((i) => i.user.id === id)?.user.name ?? id ?? 'стажёр';
  const inbox = pending.data ?? [];
  const openSignals = (signals.data ?? []).filter((s) => !s.done).length;

  return (
    <aside className="sheet" aria-label="Кабинет команды">
      <header className="sheet__head">
        <h2>Моя команда</h2>
        <span className="sheet__meta">
          {canDecide ? 'руководитель: согласование заявок' : 'наставник: прогресс стажёров'}
        </span>
      </header>

      {summary.data && (
        <dl className="summary" aria-label="Метрики команды">
          {[
            { label: 'Стажёров', value: String(summary.data.interns) },
            { label: 'Средний прогресс', value: `${Math.round(summary.data.avg_percent_done)}%` },
            { label: 'Заявок ждут решения', value: String(summary.data.actions.pending ?? 0) },
            { label: 'Вопросов без ответа', value: String(summary.data.unanswered_topics) },
          ].map((t) => (
            <div key={t.label} className="summary__item">
              <dd className="summary__value">{t.value}</dd>
              <dt className="summary__label">{t.label}</dt>
            </div>
          ))}
        </dl>
      )}

      {signals.data && (
        <section className="block">
          <h3 className="block__title">
            Стажёры просят помощи
            {openSignals > 0 && <span className="badge badge--pending signals__count">{openSignals}</span>}
          </h3>
          {signals.data.length === 0 ? (
            <p className="block__note">Сообщений нет: стажёры справляются сами.</p>
          ) : (
            <ul className="signals">
              {signals.data.slice(0, 6).map((s) => (
                <li key={s.id} className={`signal${s.done ? ' signal--done' : ''}`}>
                  <div className="signal__head">
                    <span className="signal__who">{nameOf(s.user_id)}</span>
                    <span className="signal__topic">{TOPIC_LABELS[s.topic] ?? TOPIC_LABELS.other}</span>
                    {s.at && <span className="signal__at">{timeLabel(s.at)}</span>}
                  </div>
                  <p className="signal__text">{s.text}</p>
                  {!s.done ? (
                    <button
                      className="btn btn--link"
                      onClick={() => resolve.mutate(s.id)}
                      disabled={resolve.isPending}
                      aria-label={`Отметить ${s.id} разобранным`}
                    >
                      Разобрано
                    </button>
                  ) : (
                    <span className="muted">разобрано</span>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>
      )}

      <section className="block">
        <h3 className="block__title">Заявки на согласование</h3>
        {inbox.length === 0 && <p className="block__note">Новых заявок нет.</p>}
        <ul className="ledger">
          {inbox.map((a) => (
            <li key={a.id} className="ledger__row ledger__row--request">
              <div className="ledger__main">
                <span className="ledger__title">{actionTitle(a)}</span>
                <span className="ledger__id">{a.id}</span>
                <span className="ledger__who">{nameOf(a.user_id)}</span>
              </div>
              {canDecide && (
                <>
                  <input
                    className="comment"
                    aria-label={`Комментарий к заявке ${a.id}`}
                    placeholder="Комментарий стажёру, необязательно"
                    value={comments[a.id] ?? ''}
                    onChange={(e) => setComment(a.id, e.target.value)}
                    maxLength={500}
                  />
                  <div className="ledger__actions">
                    <button
                      className="btn btn--small btn--primary"
                      onClick={() => decide.mutate({ id: a.id, decision: 'approve', comment: comments[a.id] ?? '' })}
                      disabled={decide.isPending}
                      aria-label={`Согласовать ${a.id}`}
                    >
                      Согласовать
                    </button>
                    <button
                      className="btn btn--small btn--ghost"
                      onClick={() => decide.mutate({ id: a.id, decision: 'reject', comment: comments[a.id] ?? '' })}
                      disabled={decide.isPending}
                      aria-label={`Отклонить ${a.id}`}
                    >
                      Отклонить
                    </button>
                  </div>
                </>
              )}
            </li>
          ))}
        </ul>
        {decide.isError && (
          <div className="error" role="alert">
            {(decide.error as Error).message}
          </div>
        )}
      </section>

      <section className="block">
        <h3 className="block__title">Стажёры команды</h3>
        {interns.length === 0 && <p className="block__note">В команде пока нет стажёров.</p>}
        <ul className="interns">
          {interns.map((i) => {
            const waiting = i.items.filter((it) => !it.done && it.status === 'waiting').length;
            return (
              <li key={i.user.id} className="intern">
                <div className="intern__head">
                  <span className="intern__name">{i.user.name}</span>
                  <span className="intern__pct">{Math.round(i.percent_done)}%</span>
                </div>
                {/* полоска из шагов маршрута: видно, сколько пройдено и что ждёт заявки */}
                <div
                  className="track"
                  role="progressbar"
                  aria-label={`Прогресс: ${i.user.name}`}
                  aria-valuenow={Math.round(i.percent_done)}
                  aria-valuemin={0}
                  aria-valuemax={100}
                >
                  {i.items.map((it) => (
                    <span key={it.id} className={segmentClass(it)} title={it.title} />
                  ))}
                </div>
                <p className="intern__next">
                  {i.next_step ? `Следующий шаг: ${i.next_step}` : 'Все шаги выполнены'}
                  {waiting > 0 && `; ждёт заявок: ${waiting}`}
                </p>
              </li>
            );
          })}
        </ul>
      </section>

      <section className="block">
        <h3 className="block__title">Чего нет в базе знаний</h3>
        {gaps.isError ? (
          <div className="error" role="alert">
            Не удалось загрузить вопросы без ответа.{' '}
            <button className="btn btn--link" onClick={() => void gaps.refetch()}>
              Повторить
            </button>
          </div>
        ) : gaps.isLoading ? (
          <p className="block__note">Загружаю вопросы без ответа…</p>
        ) : (gaps.data ?? []).length === 0 ? (
          <p className="block__note">Помощник пока ответил на все вопросы стажёров.</p>
        ) : (
          <>
            <p className="block__note">
              Вопросы, на которые помощник не нашёл ответа. Автор не сохраняется, персональные данные скрыты.
            </p>
            <ul className="gaps">
              {(gaps.data ?? []).slice(0, 5).map((g) => (
                <li key={g.question} className="gap">
                  <span className="gap__q">{g.question}</span>
                  <span className="gap__count" title="Сколько раз спрашивали">
                    ×{g.count}
                  </span>
                </li>
              ))}
            </ul>
          </>
        )}
      </section>
    </aside>
  );
}
