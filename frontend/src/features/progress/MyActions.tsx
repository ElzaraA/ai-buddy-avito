import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, ApiError } from '../../api/client';
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

/**
 * Мои заявки: статус, комментарий руководителя и отмена.
 * Черновики (из чата или от внешнего агента через MCP) показываем отдельным блоком:
 * заявка уходит руководителю только после подтверждения стажёром здесь или в чате.
 */
export function MyActions() {
  const user = useChatStore((s) => s.user);
  const qc = useQueryClient();
  const { data } = useQuery({
    queryKey: ['actions', user],
    queryFn: api.listActions,
    refetchInterval: ACTIONS_REFRESH_MS,
  });
  const refresh = () => {
    qc.invalidateQueries({ queryKey: ['actions', user] });
    qc.invalidateQueries({ queryKey: ['progress', user] });
  };
  // 409: заявку уже обработали в другом окне — перечитываем список, чтобы не показывать устаревшее
  const onError = (e: Error) => {
    if (e instanceof ApiError && e.status === 409) refresh();
  };
  const cancel = useMutation({ mutationFn: api.cancelAction, onSuccess: refresh, onError });
  const confirm = useMutation({ mutationFn: api.confirmAction, onSuccess: refresh, onError });
  const busy = cancel.isPending || confirm.isPending;

  const all = data ?? [];
  const drafts = all.filter((a) => a.status === 'draft');
  const sent = all.filter((a) => a.status !== 'draft');
  if (all.length === 0) return null;

  return (
    <>
      {drafts.length > 0 && (
        <div className="stage" aria-label="Ждут вашего подтверждения">
          <h3>Ждут вашего подтверждения</h3>
          <div className="muted">Черновики, подготовленные в чате или внешним агентом (MCP). Без подтверждения никуда не уходят.</div>
          <ul>
            {drafts.map((a) => (
              <li key={a.id} className="step">
                <span className="step__title">
                  {actionTitle(a)} <span className="muted">· {a.id}</span>
                </span>
                <button
                  className="link"
                  onClick={() => confirm.mutate(a.id)}
                  disabled={busy}
                  aria-label={`Отправить заявку ${a.id}`}
                >
                  отправить
                </button>
                <button
                  className="link"
                  onClick={() => cancel.mutate(a.id)}
                  disabled={busy}
                  aria-label={`Удалить черновик ${a.id}`}
                >
                  отменить
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
      {sent.length > 0 && (
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
                    disabled={busy}
                    aria-label={`Отменить заявку ${a.id}`}
                  >
                    отменить
                  </button>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
      {(cancel.isError || confirm.isError) && (
        <div className="error" role="alert">
          {((cancel.error ?? confirm.error) as Error).message}
        </div>
      )}
    </>
  );
}
