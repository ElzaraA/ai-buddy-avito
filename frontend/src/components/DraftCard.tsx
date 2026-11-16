import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { Absence } from '../api/schemas';
import { formatDate, resourceLabel, statusLabel } from './format';
import { useGuard } from './useGuard';

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
      // «Первая заявка» загорается сразу, а не через интервал опроса
      qc.invalidateQueries({ queryKey: ['achievements'] });
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
  // один замок на обе кнопки: двойной клик или «Отправить» + «Отменить» подряд не уходят дважды
  const guard = useGuard();
  const onConfirm = () => guard((done) => confirm.mutate(undefined, { onSettled: done }));
  const onCancel = () =>
    actionId ? guard((done) => cancel.mutate(undefined, { onSettled: done })) : setCancelled(true);
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
              <dd>{resourceLabel(a.resource ?? args.resource)}</dd>
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
        <div className="next">
          <div className="next__title">Что дальше</div>
          <ol className="next__list">
            {isAccess ? (
              <>
                <li>Руководитель получит заявку и согласует доступ — писать ему не нужно.</li>
                <li>Шаг «Получить доступы» в разделе «Мой план» закроется сам.</li>
              </>
            ) : (
              <>
                <li>Руководитель получит заявку и согласует её — писать ему не нужно.</li>
                <li>Коротко предупредите наставника, что вас не будет в эти дни.</li>
              </>
            )}
            <li>Статус заявки смотрите в разделе «Мои заявки» — он обновится сам.</li>
          </ol>
        </div>
      </div>
    );
  }

  if (cancelled) return <div className="msg msg--muted">Черновик отменён.</div>;

  return (
    <div className="form form--draft">
      <div className="form__title">
        {isAccess
          ? `Черновик заявки на доступ${args.resource ? ` к ${resourceLabel(args.resource)}` : ''}`
          : 'Черновик заявки на отгул'}
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
          <p className="form__hint">
            Проверьте данные и нажмите «Отправить заявку». Пока вы не нажали, заявка никуда не уходит.
          </p>
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
            <button className="btn btn--primary" onClick={onConfirm} disabled={busy}>
              {confirm.isPending ? 'Отправляю…' : 'Отправить заявку'}
            </button>
            <button className="btn btn--ghost" onClick={onCancel} disabled={busy}>
              Отменить черновик
            </button>
          </div>
        </>
      ) : isAccess ? (
        <p className="form__hint">Не получилось подготовить заявку. Напишите ещё раз, например «Запроси доступ в GitLab».</p>
      ) : (
        <p className="form__hint">Напишите дату отгула, например «в пятницу» или «завтра».</p>
      )}
    </div>
  );
}
