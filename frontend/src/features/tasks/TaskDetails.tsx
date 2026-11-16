import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError, api } from '../../api/client';
import type { Task } from '../../api/schemas';
import { formatDate } from '../../components/format';
import { useChatStore } from '../../store/chat';
import { Link } from '../shell/Link';
import { TASK_STATUS, difficultyLabel, estimateLabel } from './taskText';

type Op = 'start' | 'complete' | 'skip' | 'reopen';

const ACTIONS: Record<Task['status'], { op: Op; label: string; primary?: boolean }[]> = {
  todo: [
    { op: 'start', label: 'Взять в работу', primary: true },
    { op: 'skip', label: 'Отложить' },
  ],
  in_progress: [
    { op: 'complete', label: 'Отметить сделанной', primary: true },
    { op: 'skip', label: 'Отложить' },
  ],
  skipped: [
    { op: 'start', label: 'Взять в работу', primary: true },
    { op: 'reopen', label: 'Вернуть в список' },
  ],
  done: [],
};

const DONE_TEXT: Record<Op, string> = {
  start: 'Задача в работе. Застрянете больше чем на 20 минут — спросите Бадди или наставника.',
  complete: 'Готово! Навыки обновлены — рекомендации подстроятся.',
  skip: 'Задача отложена: в рекомендациях её не будет.',
  reopen: 'Задача снова в списке.',
};

/** Экран задачи: что сделать, критерий готовности, статья базы знаний и смена статуса. */
export function TaskDetails({ taskId, onAsk }: { taskId: string; onAsk: (q: string) => void }) {
  const user = useChatStore((s) => s.user);
  const qc = useQueryClient();
  const task = useQuery({ queryKey: ['task', user, taskId], queryFn: () => api.task(taskId), retry: false });
  const move = useMutation({
    mutationFn: (op: Op) => api.moveTask(taskId, op),
    onSuccess: (t) => {
      qc.setQueryData(['task', user, taskId], t);
      qc.invalidateQueries({ queryKey: ['tasks'] });
      qc.invalidateQueries({ queryKey: ['recommendations'] });
    },
    // конфликт (задачу уже поменяли в другой вкладке) — показываем актуальное состояние
    onError: (e) => {
      if (e instanceof ApiError && e.status === 409) void task.refetch();
    },
  });

  if (task.isLoading) return <section className="sheet sheet--state" role="status">Загружаю задачу…</section>;
  if (task.isError || !task.data) {
    const missing = task.error instanceof ApiError && task.error.status === 404;
    return (
      <section className="sheet sheet--state">
        <p className={missing ? undefined : 'error'} role={missing ? 'status' : 'alert'}>
          {missing ? 'Такой задачи нет в каталоге вашей команды.' : 'Не удалось загрузить задачу.'}
        </p>
        {missing ? (
          <Link to="/tasks" className="btn btn--link">
            К списку задач
          </Link>
        ) : (
          <button type="button" className="btn btn--link" onClick={() => void task.refetch()}>
            Повторить
          </button>
        )}
      </section>
    );
  }
  const t = task.data;
  const lastOp = move.isSuccess ? move.variables : undefined;

  return (
    <article className="sheet sheet--wide task" aria-labelledby="task-title">
      <Link to="/tasks" className="btn btn--link back-link">
        ← Все задачи
      </Link>
      <header className="sheet__head">
        <h2 id="task-title">{t.title}</h2>
        <span className={`badge badge--task-${t.status}`}>{TASK_STATUS[t.status]}</span>
      </header>
      <p className="task__summary">{t.summary}</p>
      <ul className="chips" aria-label="Параметры задачи">
        <li className="chip chip--light">{difficultyLabel(t.difficulty)}</li>
        {t.estimate_hours > 0 && <li className="chip chip--light">{estimateLabel(t.estimate_hours)}</li>}
        {t.tags.map((tag) => (
          <li key={tag} className="chip chip--blue">
            {tag}
          </li>
        ))}
      </ul>

      <section className="block" aria-labelledby="task-steps">
        <h3 className="block__title" id="task-steps">
          Что сделать
        </h3>
        <ol className="task__steps">
          {t.steps.map((s) => (
            <li key={s}>{s}</li>
          ))}
        </ol>
      </section>
      <section className="block" aria-labelledby="task-dod">
        <h3 className="block__title" id="task-dod">
          Когда задача сделана
        </h3>
        <p>{t.definition_of_done}</p>
      </section>

      <div className="task__links">
        {t.knowledge && (
          <Link to={`/knowledge/${encodeURIComponent(t.knowledge)}`} className="btn btn--ghost">
            Прочитать в базе знаний
          </Link>
        )}
        <button type="button" className="btn btn--ghost" onClick={() => onAsk(`Как подступиться к задаче «${t.title}»?`)}>
          Спросить Бадди
        </button>
      </div>

      {(t.started_at || t.done_at) && (
        <p className="muted">
          {t.started_at && `Взята ${formatDate(t.started_at.slice(0, 10))}`}
          {t.done_at && ` · сделана ${formatDate(t.done_at.slice(0, 10))}`}
        </p>
      )}

      {ACTIONS[t.status].length > 0 && (
        <div className="form__actions">
          {ACTIONS[t.status].map((a) => (
            <button
              key={a.op}
              type="button"
              className={`btn ${a.primary ? 'btn--primary' : 'btn--ghost'}`}
              disabled={move.isPending}
              onClick={() => move.mutate(a.op)}
            >
              {a.label}
            </button>
          ))}
        </div>
      )}
      {lastOp && (
        <p className="note" role="status">
          {DONE_TEXT[lastOp]}
        </p>
      )}
      {move.isError && (
        <p className="error" role="alert">
          {(move.error as Error).message}
        </p>
      )}
    </article>
  );
}
