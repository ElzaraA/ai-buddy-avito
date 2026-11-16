import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, ApiError } from '../../api/client';
import type { Absence } from '../../api/schemas';
import { formatDate, resourceLabel, statusLabel } from '../../components/format';
import { useChatStore } from '../../store/chat';

/** Как часто перечитываем заявки: решение руководителя появляется без перезагрузки страницы. */
export const ACTIONS_REFRESH_MS = 5000;

/** Статусы заявок, которые действительно отправлены руководителю. */
export const SENT_STATUSES = ['pending', 'approved', 'rejected'];

export function actionTitle(a: Absence) {
  if (a.type === 'access') return `Доступ к ${resourceLabel(a.resource) || 'системе'}`;
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
  const { data, isLoading, isError, refetch } = useQuery({
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

  if (isLoading) return <p className="muted" role="status">Загружаю заявки…</p>;
  if (isError) {
    return (
      <div className="error" role="alert">
        Не удалось загрузить заявки.{' '}
        <button className="btn btn--link" onClick={() => void refetch()}>Повторить</button>
      </div>
    );
  }

  const all = data ?? [];
  const drafts = all.filter((a) => a.status === 'draft');
  // «Отправленные» — только то, что реально ушло руководителю; отменённые — отдельно и приглушённо
  const sent = all.filter((a) => SENT_STATUSES.includes(a.status));
  const cancelled = all.filter((a) => a.status === 'cancelled');
  if (all.length === 0) return null;

  return (
    <>
      {drafts.length > 0 && (
        <section className="block" aria-label="Ждут вашего подтверждения">
          <h3 className="block__title">Ждут вашего подтверждения</h3>
          <p className="block__note">
            Черновики из чата или от внешнего агента. Пока вы их не отправите, они никуда не уходят.
          </p>
          <ul className="ledger">
            {drafts.map((a) => (
              <li key={a.id} className="ledger__row ledger__row--draft">
                <div className="ledger__main">
                  <span className="ledger__title">{actionTitle(a)}</span>
                  <span className="ledger__id">{a.id}</span>
                </div>
                <div className="ledger__actions">
                  <button
                    className="btn btn--small btn--primary"
                    onClick={() => confirm.mutate(a.id)}
                    disabled={busy}
                    aria-label={`Отправить заявку ${a.id}`}
                  >
                    Отправить
                  </button>
                  <button
                    className="btn btn--small btn--ghost"
                    onClick={() => cancel.mutate(a.id)}
                    disabled={busy}
                    aria-label={`Отменить черновик ${a.id}`}
                  >
                    Отменить черновик
                  </button>
                </div>
              </li>
            ))}
          </ul>
        </section>
      )}
      {sent.length > 0 && (
        <section className="block" aria-label="Отправленные">
          {/* «Мои заявки» — уже заголовок раздела; здесь — только отправленные руководителю */}
          <h3 className="block__title">Отправленные</h3>
          <ul className="ledger">
            {sent.map((a) => (
              <li key={a.id} className="ledger__row">
                <div className="ledger__main">
                  <span className="ledger__title">{actionTitle(a)}</span>
                  <span className="ledger__id">{a.id}</span>
                  {decisionComment(a) && <span className="ledger__comment">«{decisionComment(a)}»</span>}
                </div>
                <div className="ledger__actions">
                  <span className={`badge badge--${a.status}`}>{statusLabel(a.status)}</span>
                  {a.status === 'pending' && (
                    <button
                      className="btn btn--link"
                      onClick={() => cancel.mutate(a.id)}
                      disabled={busy}
                      aria-label={`Отменить заявку ${a.id}`}
                    >
                      Отменить заявку
                    </button>
                  )}
                </div>
              </li>
            ))}
          </ul>
        </section>
      )}
      {cancelled.length > 0 && (
        <section className="block block--muted" aria-label="Отменённые">
          <h3 className="block__title">Отменённые</h3>
          <ul className="ledger">
            {cancelled.map((a) => (
              <li key={a.id} className="ledger__row ledger__row--muted">
                <div className="ledger__main">
                  <span className="ledger__title">{actionTitle(a)}</span>
                  <span className="ledger__id">{a.id}</span>
                </div>
                <span className="badge badge--cancelled">{statusLabel(a.status)}</span>
              </li>
            ))}
          </ul>
        </section>
      )}
      {(cancel.isError || confirm.isError) && (
        <div className="error" role="alert">
          {((cancel.error ?? confirm.error) as Error).message}
        </div>
      )}
    </>
  );
}
