import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { Recommendation, Task } from '../../api/schemas';
import { useChatStore } from '../../store/chat';
import { Link } from '../shell/Link';
import { TASK_STATUS, difficultyLabel, estimateLabel } from './taskText';

type Filter = 'all' | Task['status'];
const FILTERS: { id: Filter; label: string }[] = [
  { id: 'all', label: 'Все' },
  { id: 'todo', label: 'Не начаты' },
  { id: 'in_progress', label: 'В работе' },
  { id: 'done', label: 'Сделано' },
  { id: 'skipped', label: 'Отложены' },
];

/** Рекомендации модели: почему задача подходит — из тех же признаков, что видит модель. */
export function RecommendedList({
  items,
  compact = false,
  linkable = true,
}: {
  items: Recommendation[];
  compact?: boolean;
  /** наставнику раздел задач стажёра недоступен — название без ссылки */
  linkable?: boolean;
}) {
  return (
    <ol className="rec-list">
      {items.map((t, i) => (
        <li key={t.id} className="rec">
          <span className="rec__rank" aria-hidden>
            {i + 1}
          </span>
          <div className="rec__body">
            {linkable ? (
              <Link to={`/tasks/${encodeURIComponent(t.id)}`} className="rec__title">
                {t.title}
              </Link>
            ) : (
              <span className="rec__title">{t.title}</span>
            )}
            {!compact && <span className="rec__summary">{t.summary}</span>}
            <ul className="rec__why" aria-label="Почему подходит">
              {t.reasons.map((r) => (
                <li key={r}>{r}</li>
              ))}
            </ul>
          </div>
          <span className="rec__score" title="Оценка модели: насколько задача подходит вам (0–100%)">
            {Math.round(t.score * 100)}%
          </span>
        </li>
      ))}
    </ol>
  );
}

export function sourceNote(source: 'model' | 'rule_based') {
  return source === 'model'
    ? 'Подобрано моделью рекомендаций по вашим навыкам и сделанным задачам.'
    : 'Модель рекомендаций сейчас недоступна — подобрано простым правилом по навыкам и треку.';
}

/** Раздел «Задачи»: рекомендации следующей задачи и каталог задач команды со статусами. */
export function TasksView() {
  const user = useChatStore((s) => s.user);
  const [filter, setFilter] = useState<Filter>('all');
  const tasks = useQuery({ queryKey: ['tasks', user], queryFn: api.tasks });
  const recs = useQuery({ queryKey: ['recommendations', user], queryFn: () => api.recommendations(3) });

  const list = (tasks.data ?? []).filter((t) => filter === 'all' || t.status === filter);
  const count = (f: Filter) => (tasks.data ?? []).filter((t) => f === 'all' || t.status === f).length;

  return (
    <section className="sheet sheet--wide tasks" aria-label="Задачи адаптации">
      <section className="block" aria-labelledby="rec-title">
        <h2 className="block__title" id="rec-title">
          Бадди советует взять
        </h2>
        {recs.isLoading && <p className="muted">Подбираю задачи…</p>}
        {recs.isError && (
          <p className="error" role="alert">
            Не удалось получить рекомендации.{' '}
            <button type="button" className="btn btn--link" onClick={() => void recs.refetch()}>
              Повторить
            </button>
          </p>
        )}
        {recs.data && recs.data.items.length === 0 && (
          <p className="muted">Все задачи каталога уже взяты — спросите наставника, что дальше.</p>
        )}
        {recs.data && recs.data.items.length > 0 && (
          <>
            <RecommendedList items={recs.data.items} />
            <p className="block__note">{sourceNote(recs.data.source)}</p>
          </>
        )}
      </section>

      <section className="block" aria-labelledby="all-title">
        <h2 className="block__title" id="all-title">
          Все задачи команды
        </h2>
        <div className="filters" role="group" aria-label="Фильтр по статусу">
          {FILTERS.map((f) => (
            <button
              key={f.id}
              type="button"
              className={`filter${filter === f.id ? ' filter--active' : ''}`}
              aria-pressed={filter === f.id}
              onClick={() => setFilter(f.id)}
            >
              {f.label} · {count(f.id)}
            </button>
          ))}
        </div>
        {tasks.isLoading && <p className="muted">Загружаю задачи…</p>}
        {tasks.isError && (
          <p className="error" role="alert">
            Не удалось загрузить задачи.{' '}
            <button type="button" className="btn btn--link" onClick={() => void tasks.refetch()}>
              Повторить
            </button>
          </p>
        )}
        {tasks.data && list.length === 0 && <p className="muted">В этом статусе задач нет.</p>}
        <ul className="task-list">
          {list.map((t) => (
            <li key={t.id} className={`task-row task-row--${t.status}`}>
              <Link to={`/tasks/${encodeURIComponent(t.id)}`} className="task-row__title">
                {t.title}
              </Link>
              <span className="task-row__meta">
                {difficultyLabel(t.difficulty)}
                {t.estimate_hours > 0 && ` · ${estimateLabel(t.estimate_hours)}`} · {t.tags.join(', ')}
              </span>
              <span className={`badge badge--task-${t.status}`}>{TASK_STATUS[t.status]}</span>
            </li>
          ))}
        </ul>
      </section>
    </section>
  );
}
