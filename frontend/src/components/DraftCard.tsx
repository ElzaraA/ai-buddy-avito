import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { Absence } from '../api/schemas';
import { formatDate, statusLabel } from './format';

interface Props {
  args: Record<string, string>;
  text?: string;
}

/**
 * Черновик заявки на отгул. Заявка создаётся только после явного подтверждения
 * стажёра (инвариант Draft → Confirm из спецификации агента).
 */
export function DraftCard({ args, text }: Props) {
  const qc = useQueryClient();
  const [cancelled, setCancelled] = useState(false);
  const hasDates = Boolean(args.date_from);

  const confirm = useMutation<Absence>({
    mutationFn: async () => {
      const draft = await api.createAbsenceDraft({
        date_from: args.date_from,
        date_to: args.date_to || args.date_from,
        reason: args.reason,
      });
      return api.confirmAction(draft.id);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['progress'] }),
  });

  if (confirm.data) {
    const a = confirm.data;
    return (
      <div className="card card--success" role="status">
        <div className="card__title">✅ Заявка {a.id} отправлена</div>
        <div>
          Отгул {formatDate(a.date_from)}
          {a.date_to && a.date_to !== a.date_from ? ` – ${formatDate(a.date_to)}` : ''} · статус:{' '}
          <b>{statusLabel(a.status)}</b>
        </div>
        <div className="muted">Не забудьте предупредить наставника лично.</div>
      </div>
    );
  }

  if (cancelled) return <div className="card muted">Черновик отменён.</div>;

  return (
    <div className="card card--draft">
      <div className="card__title">📝 Черновик заявки на отгул</div>
      {text && <p className="card__text">{text}</p>}
      {hasDates ? (
        <>
          <div>
            Даты: <b>{formatDate(args.date_from)}</b>
            {args.date_to && args.date_to !== args.date_from && (
              <>
                {' '}– <b>{formatDate(args.date_to)}</b>
              </>
            )}
          </div>
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
      ) : (
        <div className="muted">Напишите дату отгула, например «отгул 2026-11-20».</div>
      )}
    </div>
  );
}
