import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, ApiError } from '../../api/client';
import { useChatStore } from '../../store/chat';
import { actionTitle } from '../progress/MyActions';

/** Как часто перечитываем кабинет: новые заявки и вопросы стажёров появляются сами. */
export const TEAM_REFRESH_MS = 5000;

interface Props {
  /** buddy — наставник (только просмотр), lead — руководитель (согласует заявки) */
  role: string;
}

/**
 * Кабинет наставника и руководителя: заявки на согласование, прогресс стажёров команды
 * и вопросы, на которые помощник не нашёл ответа в базе знаний.
 */
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
  // «метрики пользы»: необязательны — если сводка не загрузилась, кабинет работает без плиток
  const summary = useQuery({ queryKey: ['team-summary', user], queryFn: api.teamSummary, ...opts });

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
      <section className="panel progress">
        <div className="error" role="alert">
          Не удалось загрузить данные команды.{' '}
          <button className="link" onClick={() => void Promise.all([pending.refetch(), progress.refetch()])}>
            Повторить
          </button>
        </div>
      </section>
    );
  if (pending.isLoading || progress.isLoading)
    return <section className="panel progress">Загружаю данные команды…</section>;

  const interns = progress.data ?? [];
  const nameOf = (id?: string) => interns.find((i) => i.user.id === id)?.user.name ?? id ?? 'стажёр';
  const inbox = pending.data ?? [];

  return (
    <section className="panel progress" aria-label="Кабинет команды">
      <header className="panel__header">
        <h2>Моя команда</h2>
        <span className="muted">{canDecide ? 'руководитель: согласование заявок' : 'наставник: прогресс стажёров'}</span>
      </header>

      {summary.data && (
        <div className="tiles" aria-label="Метрики команды">
          {[
            { label: 'Стажёров', value: String(summary.data.interns) },
            { label: 'Средний прогресс', value: `${Math.round(summary.data.avg_percent_done)}%` },
            { label: 'Заявок ждут решения', value: String(summary.data.actions.pending ?? 0) },
            { label: 'Вопросов без ответа', value: String(summary.data.unanswered_topics) },
          ].map((t) => (
            <div key={t.label} className="tile">
              <div className="tile__value">{t.value}</div>
              <div className="tile__label">{t.label}</div>
            </div>
          ))}
        </div>
      )}

      <div className="stage">
        <h3>Заявки на согласование</h3>
        {inbox.length === 0 && <div className="muted">Новых заявок нет.</div>}
        <ul>
          {inbox.map((a) => (
            <li key={a.id} className="request">
              <div>
                <b>{actionTitle(a)}</b> <span className="muted">· {a.id}</span>
                <div className="muted">{nameOf(a.user_id)}</div>
              </div>
              {canDecide && (
                <input
                  className="comment"
                  aria-label={`Комментарий к заявке ${a.id}`}
                  placeholder="Комментарий стажёру к этой заявке (необязательно)"
                  value={comments[a.id] ?? ''}
                  onChange={(e) => setComment(a.id, e.target.value)}
                  maxLength={500}
                />
              )}
              {canDecide && (
                <div className="card__actions">
                  <button
                    onClick={() => decide.mutate({ id: a.id, decision: 'approve', comment: comments[a.id] ?? '' })}
                    disabled={decide.isPending}
                    aria-label={`Согласовать ${a.id}`}
                  >
                    Согласовать
                  </button>
                  <button
                    className="secondary"
                    onClick={() => decide.mutate({ id: a.id, decision: 'reject', comment: comments[a.id] ?? '' })}
                    disabled={decide.isPending}
                    aria-label={`Отклонить ${a.id}`}
                  >
                    Отклонить
                  </button>
                </div>
              )}
            </li>
          ))}
        </ul>
        {decide.isError && (
          <div className="error" role="alert">
            {(decide.error as Error).message}
          </div>
        )}
      </div>

      <div className="stage">
        <h3>Стажёры команды</h3>
        {interns.length === 0 && <div className="muted">В команде пока нет стажёров.</div>}
        <ul>
          {interns.map((i) => {
            const waiting = i.items.filter((it) => !it.done && it.status === 'waiting').length;
            return (
              <li key={i.user.id} className="intern">
                <div className="intern__head">
                  <span>{i.user.name}</span>
                  <b>{Math.round(i.percent_done)}%</b>
                </div>
                <div
                  className="progress__bar"
                  role="progressbar"
                  aria-label={`Прогресс: ${i.user.name}`}
                  aria-valuenow={Math.round(i.percent_done)}
                  aria-valuemin={0}
                  aria-valuemax={100}
                >
                  <div style={{ width: `${i.percent_done}%` }} />
                </div>
                <div className="muted">
                  {i.next_step ? `Следующий шаг: ${i.next_step}` : 'Все шаги выполнены'}
                  {waiting > 0 && ` · ждёт заявок: ${waiting}`}
                </div>
              </li>
            );
          })}
        </ul>
      </div>

      <div className="stage">
        <h3>Чего нет в базе знаний</h3>
        {gaps.isError ? (
          <div className="error" role="alert">
            Не удалось загрузить вопросы без ответа.{' '}
            <button className="link" onClick={() => void gaps.refetch()}>
              Повторить
            </button>
          </div>
        ) : gaps.isLoading ? (
          <div className="muted">Загружаю вопросы без ответа…</div>
        ) : (gaps.data ?? []).length === 0 ? (
          <div className="muted">Помощник пока ответил на все вопросы стажёров.</div>
        ) : (
          <>
            <div className="muted">
              Вопросы, на которые помощник не нашёл ответа (без автора, персональные данные маскируются):
            </div>
            <ul>
              {(gaps.data ?? []).slice(0, 5).map((g) => (
                <li key={g.question} className="step">
                  <span className="step__title">{g.question}</span>
                  <span className="badge">×{g.count}</span>
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
    </section>
  );
}
