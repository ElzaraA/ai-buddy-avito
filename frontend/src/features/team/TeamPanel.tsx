import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
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
  const [comment, setComment] = useState('');

  const opts = { refetchInterval: TEAM_REFRESH_MS };
  const pending = useQuery({ queryKey: ['team-pending', user], queryFn: api.teamPending, ...opts });
  const progress = useQuery({ queryKey: ['team-progress', user], queryFn: api.teamProgress, ...opts });
  const gaps = useQuery({ queryKey: ['team-gaps', user], queryFn: api.knowledgeGaps, ...opts });

  const decide = useMutation({
    mutationFn: (v: { id: string; decision: 'approve' | 'reject' }) => api.decideAction(v.id, v.decision, comment.trim()),
    onSuccess: () => {
      setComment('');
      qc.invalidateQueries({ queryKey: ['team-pending', user] });
      qc.invalidateQueries({ queryKey: ['team-progress', user] });
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
                <div className="card__actions">
                  <button
                    onClick={() => decide.mutate({ id: a.id, decision: 'approve' })}
                    disabled={decide.isPending}
                    aria-label={`Согласовать ${a.id}`}
                  >
                    Согласовать
                  </button>
                  <button
                    className="secondary"
                    onClick={() => decide.mutate({ id: a.id, decision: 'reject' })}
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
        {canDecide && inbox.length > 0 && (
          <input
            className="comment"
            aria-label="Комментарий к решению"
            placeholder="Комментарий стажёру (необязательно)"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            maxLength={500}
          />
        )}
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
        {(gaps.data ?? []).length === 0 ? (
          <div className="muted">Помощник пока ответил на все вопросы стажёров.</div>
        ) : (
          <>
            <div className="muted">Вопросы, на которые помощник не нашёл ответа (обезличенно):</div>
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
