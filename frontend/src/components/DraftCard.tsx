import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { Absence } from '../api/schemas';
import { formatDate, statusLabel } from './format';

interface Props {
  /** absence | access */
  action?: string;
  args: Record<string, string>;
  text?: string;
}

/**
 * Черновик заявки. Заявка уходит только после явного подтверждения стажёра
 * (инвариант Draft → Confirm из спецификации агента).
 * Если бэк уже сохранил черновик и вернул args.action_id — подтверждаем его,
 * не создавая второй черновик.
 */
export function DraftCard({ action = 'absence', args, text }: Props) {
  const qc = useQueryClient();
  const [cancelled, setCancelled] = useState(false);
  const isAccess = action === 'access';
  const actionId = args.action_id;
  // доступ без номера черновика подтвердить нечем: POST /actions создаёт только отгулы
  const ready = Boolean(actionId) || (!isAccess && Boolean(args.date_from));

  const confirm = useMutation<Absence>({
    mutationFn: async () => {
      if (actionId) return api.confirmAction(actionId);
      const draft = await api.createAbsenceDraft({
        date_from: args.date_from,
        date_to: args.date_to || args.date_from,
        reason: args.reason,
      });
      return api.confirmAction(draft.id);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['progress'] });
      qc.invalidateQueries({ queryKey: ['actions'] });
    },
  });

  if (confirm.data) {
    const a = confirm.data;
    return (
      <div className="card card--success" role="status">
        <div className="card__title">✅ Заявка {a.id} отправлена</div>
        {a.type === 'access' ? (
          <div>
            Доступ к {a.resource ?? args.resource} · статус: <b>{statusLabel(a.status)}</b>
          </div>
        ) : (
          <div>
            Отгул {formatDate(a.date_from)}
            {a.date_to && a.date_to !== a.date_from ? ` – ${formatDate(a.date_to)}` : ''} · статус:{' '}
            <b>{statusLabel(a.status)}</b>
          </div>
        )}
        {!isAccess && <div className="muted">Не забудьте предупредить наставника лично.</div>}
      </div>
    );
  }

  if (cancelled) return <div className="card muted">Черновик отменён.</div>;

  return (
    <div className="card card--draft">
      <div className="card__title">
        📝 {isAccess ? `Черновик заявки на доступ${args.resource ? ` к ${args.resource}` : ''}` : 'Черновик заявки на отгул'}
      </div>
      {text && <p className="card__text">{text}</p>}
      {ready ? (
        <>
          {!isAccess && args.date_from && (
            <div>
              Даты: <b>{formatDate(args.date_from)}</b>
              {args.date_to && args.date_to !== args.date_from && (
                <>
                  {' '}– <b>{formatDate(args.date_to)}</b>
                </>
              )}
            </div>
          )}
          {confirm.isError && (
            <div className="error" role="alert">
              Не удалось отправить: {(confirm.error as Error).message}
            </div>
          )}
          <div className="card__actions">
            <button onClick={() => confirm.mutate()} disabled={confirm.isPending}>
              {confirm.isPending ? 'Отправляю…' : 'Отправить заявку'}
            </button>
            <button className="secondary" onClick={() => setCancelled(true)} disabled={confirm.isPending}>
              Отменить
            </button>
          </div>
        </>
      ) : isAccess ? (
        <div className="muted">Сервер не вернул номер черновика — попробуйте повторить запрос.</div>
      ) : (
        <div className="muted">Напишите дату отгула, например «2026-11-20» или «в пятницу».</div>
      )}
    </div>
  );
}
