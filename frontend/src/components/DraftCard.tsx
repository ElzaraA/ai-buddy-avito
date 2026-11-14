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

function datesLabel(from: string, to?: string) {
  return to && to !== from ? `${formatDate(from)} – ${formatDate(to)}` : formatDate(from);
}

/**
 * Черновик заявки. Заявка уходит только после явного подтверждения стажёра
 * (инвариант Draft → Confirm из спецификации агента).
 * Если бэк уже сохранил черновик и вернул args.action_id — подтверждаем его,
 * не создавая второй черновик.
 * Визуально — бланк с пунктирной рамкой: пока не отправлен, это ещё не заявка.
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

  // черновик уже сохранён на сервере — отменяем и там, иначе он останется висеть в «Ждут подтверждения»
  const cancel = useMutation({
    mutationFn: () => api.cancelAction(actionId),
    onSuccess: () => {
      setCancelled(true);
      qc.invalidateQueries({ queryKey: ['actions'] });
    },
  });
  const onCancel = () => (actionId ? cancel.mutate() : setCancelled(true));
  const busy = confirm.isPending || cancel.isPending;

  if (confirm.data) {
    const a = confirm.data;
    return (
      <div className="form form--sent" role="status">
        <div className="form__title">Заявка {a.id} отправлена</div>
        <dl className="kv">
          {a.type === 'access' ? (
            <div>
              <dt>Доступ</dt>
              <dd>{a.resource ?? args.resource}</dd>
            </div>
          ) : (
            <div>
              <dt>Отгул</dt>
              <dd>{datesLabel(a.date_from, a.date_to)}</dd>
            </div>
          )}
          <div>
            <dt>Статус</dt>
            <dd>{statusLabel(a.status)}</dd>
          </div>
        </dl>
        {!isAccess && <p className="form__hint">Предупредите наставника лично.</p>}
      </div>
    );
  }

  if (cancelled) return <div className="msg msg--muted">Черновик отменён.</div>;

  return (
    <div className="form form--draft">
      <div className="form__title">
        {isAccess ? `Черновик заявки на доступ${args.resource ? ` к ${args.resource}` : ''}` : 'Черновик заявки на отгул'}
      </div>
      {text && <p className="form__text">{text}</p>}
      {ready ? (
        <>
          {!isAccess && args.date_from && (
            <dl className="kv">
              <div>
                <dt>Даты</dt>
                <dd>{datesLabel(args.date_from, args.date_to)}</dd>
              </div>
            </dl>
          )}
          {confirm.isError && (
            <div className="error" role="alert">
              Не удалось отправить: {(confirm.error as Error).message}
            </div>
          )}
          {cancel.isError && (
            <div className="error" role="alert">
              Не удалось отменить: {(cancel.error as Error).message}
            </div>
          )}
          <div className="form__actions">
            <button className="btn btn--primary" onClick={() => confirm.mutate()} disabled={busy}>
              {confirm.isPending ? 'Отправляю…' : 'Отправить заявку'}
            </button>
            <button className="btn btn--ghost" onClick={onCancel} disabled={busy}>
              Отменить
            </button>
          </div>
        </>
      ) : isAccess ? (
        <p className="form__hint">Сервер не вернул номер черновика. Повторите запрос в чате.</p>
      ) : (
        <p className="form__hint">Напишите дату отгула, например «в пятницу» или «завтра».</p>
      )}
    </div>
  );
}
