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

/** Состояние этапа на маршруте: пройден, текущий (в нём следующий шаг) или впереди. */
function stationState(items: ChecklistItem[], nextStep: string) {
  if (items.every((i) => i.done)) return 'done';
  if (items.some((i) => i.title === nextStep)) return 'current';
  return 'ahead';
}

function stepClass(it: ChecklistItem, nextStep: string) {
  if (it.done) return 'step step--done';
  if (it.status === 'waiting') return 'step step--waiting';
  if (it.title === nextStep) return 'step step--current';
  return 'step';
}

/** withActions — показывать «Мои заявки» под планом (в оболочке заявки живут в отдельном разделе). */
export function ProgressPanel({ withActions = true }: { withActions?: boolean }) {
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

  if (isLoading) return <aside className="sheet sheet--state">Загружаю план адаптации…</aside>;
  if (isError || !data)
    return (
      <aside className="sheet sheet--state">
        <div className="error" role="alert">
          Не удалось загрузить прогресс.{' '}
          <button className="btn btn--link" onClick={() => void refetch()}>
            Повторить
          </button>
        </div>
      </aside>
    );

  const pct = Math.round(data.percent_done ?? percentDone(data.items));

  return (
    <aside className="sheet" aria-label="Прогресс адаптации">
      <header className="sheet__head">
        <h2>Моя адаптация</h2>
        <span className="sheet__meta">Выполнено {pct}%</span>
      </header>

      <div className="meter" role="progressbar" aria-label="Пройдено плана адаптации" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}>
        <div style={{ width: `${pct}%` }} />
      </div>

      {data.next_step && (
        <p className="now">
          Сейчас: <span className="next__title">{data.next_step}</span>
        </p>
      )}

      <ol className="route">
        {groupByStage(data.items).map((g) => (
          <li key={g.stage} className={`station station--${stationState(g.items, data.next_step)}`}>
            <h3 className="station__name">{stageLabel(g.stage)}</h3>
            <ul className="steps">
              {g.items.map((it) => (
                <li key={it.id} className={stepClass(it, data.next_step)}>
                  <span className="step__title">{it.title}</span>
                  {!it.done && it.status === 'waiting' && (
                    <span
                      className="badge badge--pending"
                      title={it.related_action_id ? `Заявка ${it.related_action_id}` : 'Шаг ждёт обработки заявки'}
                    >
                      ждёт согласования
                    </span>
                  )}
                  {!it.done && closesByApproval(it) && (
                    <span className="step__hint">
                      {it.status === 'waiting'
                        ? 'закроется после согласования руководителем'
                        : 'запросите доступ в чате — шаг закроется после согласования'}
                    </span>
                  )}
                  {!it.done && !closesByApproval(it) && (
                    <button
                      className="btn btn--link step__done"
                      onClick={() => complete.mutate(it.id)}
                      disabled={complete.isPending}
                      aria-label={`Отметить «${it.title}» выполненным`}
                    >
                      Готово
                    </button>
                  )}
                </li>
              ))}
            </ul>
          </li>
        ))}
      </ol>

      {complete.isError && (
        <div className="error" role="alert">
          {(complete.error as Error).message}
        </div>
      )}

      {withActions && <MyActions />}
    </aside>
  );
}
