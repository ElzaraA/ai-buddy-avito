import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { Absence } from '../../api/schemas';
import { formatDate, statusLabel } from '../../components/format';
import { useChatStore } from '../../store/chat';

/** Как часто перечитываем заявки: решение руководителя появляется без перезагрузки страницы. */
export const ACTIONS_REFRESH_MS = 5000;

export function actionTitle(a: Absence) {
  if (a.type === 'access') return `Доступ к ${a.resource ?? 'системе'}`;
  const to = a.date_to && a.date_to !== a.date_from ? ` – ${formatDate(a.date_to)}` : '';
  return `Отгул ${formatDate(a.date_from)}${to}`;
}

/** Комментарий руководителя из истории заявки (последнее решение). */
export function decisionComment(a: Absence) {
  const last = [...(a.history ?? [])].reverse().find((e) => e.status === 'approved' || e.status === 'rejected');
  return last?.comment ?? '';
}

/** Мои заявки: статус, комментарий руководителя и отмена. Черновики живут в чате и здесь не показываются. */
export function MyActions() {
  const user = useChatStore((s) => s.user);
  const qc = useQueryClient();
  const { data } = useQuery({
    queryKey: ['actions', user],
    queryFn: api.listActions,
    refetchInterval: ACTIONS_REFRESH_MS,
  });
  const cancel = useMutation({
    mutationFn: api.cancelAction,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['actions', user] });
      qc.invalidateQueries({ queryKey: ['progress', user] });
    },
  });

  const sent = (data ?? []).filter((a) => a.status !== 'draft');
  if (sent.length === 0) return null;

  return (
    <div className="stage" aria-label="Мои заявки">
      <h3>Мои заявки</h3>
      <ul>
        {sent.map((a) => (
          <li key={a.id} className="step">
            <span className="step__title">
              {actionTitle(a)} <span className="muted">· {a.id}</span>
              {decisionComment(a) && <span className="muted"> · «{decisionComment(a)}»</span>}
            </span>
            <span className={`badge badge--${a.status}`}>{statusLabel(a.status)}</span>
            {a.status === 'pending' && (
              <button
                className="link"
                onClick={() => cancel.mutate(a.id)}
                disabled={cancel.isPending}
                aria-label={`Отменить заявку ${a.id}`}
              >
                отменить
              </button>
            )}
          </li>
        ))}
      </ul>
      {cancel.isError && (
        <div className="error" role="alert">
          {(cancel.error as Error).message}
        </div>
      )}
    </div>
  );
}
