import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { ChecklistItem } from '../../api/schemas';
import { stageLabel } from '../../components/format';
import { useChatStore } from '../../store/chat';
import { ACTIONS_REFRESH_MS, MyActions } from './MyActions';

export function groupByStage(items: ChecklistItem[]) {
  const groups = new Map<string, ChecklistItem[]>();
  for (const it of items) {
    groups.set(it.stage, [...(groups.get(it.stage) ?? []), it]);
  }
  return Array.from(groups, ([stage, list]) => ({ stage, items: list }));
}

export function percentDone(items: ChecklistItem[]) {
  if (items.length === 0) return 0;
  return Math.round((items.filter((i) => i.done).length / items.length) * 100);
}

/**
 * Шаг закрывается сам, когда руководитель согласует заявку: ручная отметка «готово»
 * обходила бы согласование, поэтому кнопку для таких шагов не показываем.
 */
export function closesByApproval(it: ChecklistItem) {
  return it.status === 'waiting' || it.kind === 'access';
}

export function ProgressPanel() {
  const user = useChatStore((s) => s.user);
  const qc = useQueryClient();
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['progress', user],
    queryFn: api.progress,
    // шаг сам станет «выполнен», когда руководитель согласует заявку на доступ
    refetchInterval: ACTIONS_REFRESH_MS,
  });

  const complete = useMutation({
    mutationFn: api.completeStep,
    onSuccess: (next) => qc.setQueryData(['progress', user], next),
  });

  if (isLoading) return <section className="panel progress">Загружаю план адаптации…</section>;
  if (isError || !data)
    return (
      <section className="panel progress">
        <div className="error" role="alert">
          Не удалось загрузить прогресс.{' '}
          <button className="link" onClick={() => void refetch()}>
            Повторить
          </button>
        </div>
      </section>
    );

  const pct = Math.round(data.percent_done ?? percentDone(data.items));

  return (
    <section className="panel progress" aria-label="Прогресс адаптации">
      <header className="panel__header">
        <h2>Моя адаптация</h2>
      </header>

      <div className="progress__bar" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}>
        <div style={{ width: `${pct}%` }} />
      </div>
      <div className="muted">Выполнено {pct}%</div>

      {data.next_step && (
        <div className="next">
          <div className="next__label">Следующий шаг</div>
          <div className="next__title">{data.next_step}</div>
        </div>
      )}

      {groupByStage(data.items).map((g) => (
        <div key={g.stage} className="stage">
          <h3>{stageLabel(g.stage)}</h3>
          <ul>
            {g.items.map((it) => (
              <li key={it.id} className={it.done ? 'step step--done' : 'step'}>
                <span className="step__icon" aria-hidden>
                  {it.done ? '✓' : '○'}
                </span>
                <span className="step__title">{it.title}</span>
                {!it.done && it.status === 'waiting' && (
                  <span
                    className="badge"
                    title={it.related_action_id ? `Заявка ${it.related_action_id}` : 'Шаг ждёт обработки заявки'}
                  >
                    ждёт согласования
                  </span>
                )}
                {!it.done && closesByApproval(it) && (
                  <span className="muted step__hint">
                    {it.status === 'waiting'
                      ? 'закроется после согласования руководителем'
                      : 'запросите доступ в чате — шаг закроется после согласования'}
                  </span>
                )}
                {!it.done && !closesByApproval(it) && (
                  <button
                    className="link"
                    onClick={() => complete.mutate(it.id)}
                    disabled={complete.isPending}
                    aria-label={`Отметить «${it.title}» выполненным`}
                  >
                    готово
                  </button>
                )}
              </li>
            ))}
          </ul>
        </div>
      ))}

      {complete.isError && (
        <div className="error" role="alert">
          {(complete.error as Error).message}
        </div>
      )}

      <MyActions />
    </section>
  );
}
