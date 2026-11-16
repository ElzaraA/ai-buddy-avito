import { FormEvent, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { Absence } from '../../api/schemas';
import { DraftCard } from '../../components/DraftCard';
import { Link } from '../shell/Link';
import { navigate } from '../shell/views';

const SYSTEMS = [
  { id: 'gitlab', label: 'GitLab (Git)' },
  { id: 'jira', label: 'Jira' },
  { id: 'grafana', label: 'Grafana' },
];
const MAX_REASON = 500;
const MAX_DAYS = 30;

/** Сегодняшняя дата по Москве в формате поля date. */
export const todayMsk = (now = new Date()) => now.toLocaleDateString('sv-SE', { timeZone: 'Europe/Moscow' });

/** Проверка до отправки: те же правила, что на сервере, но с понятными сообщениями. */
export function validateAbsence(from: string, to: string, today = todayMsk()): string | null {
  if (!from || !to) return 'Укажите даты начала и конца.';
  if (from < today) return 'Дата начала уже прошла.';
  if (to < from) return 'Дата конца раньше даты начала.';
  const days = (Date.parse(to) - Date.parse(from)) / 86_400_000;
  if (days > MAX_DAYS) return `Отгул длиннее ${MAX_DAYS} дней оформляется через руководителя.`;
  return null;
}

/**
 * Новая заявка без чата: форма создаёт черновик на сервере, а отправить его руководителю можно только
 * явным подтверждением (тот же инвариант Draft → Confirm, что у черновиков из чата).
 */
export function NewRequest({ type }: { type: 'access' | 'absence' }) {
  const qc = useQueryClient();
  const [resource, setResource] = useState('gitlab');
  const [from, setFrom] = useState(todayMsk());
  const [to, setTo] = useState(todayMsk());
  const [reason, setReason] = useState('');
  const [invalid, setInvalid] = useState<string | null>(null);
  const [draft, setDraft] = useState<Absence | null>(null);

  const create = useMutation({
    mutationFn: () =>
      type === 'access'
        ? api.createAccessDraft({ resource, reason: reason.trim() })
        : api.createAbsenceDraft({ date_from: from, date_to: to, reason: reason.trim() }),
    onSuccess: (d) => {
      setDraft(d);
      qc.invalidateQueries({ queryKey: ['actions'] });
    },
  });

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const err = type === 'absence' ? validateAbsence(from, to) : null;
    setInvalid(err);
    if (!err) create.mutate();
  };

  const tab = (t: 'access' | 'absence', label: string) => (
    <Link
      to={`/actions/new?type=${t}`}
      className={`filter${type === t ? ' filter--active' : ''}`}
      aria-current={type === t ? 'page' : undefined}
    >
      {label}
    </Link>
  );

  if (draft) {
    const args: Record<string, string> = { action_id: draft.id, date_from: draft.date_from, date_to: draft.date_to };
    if (draft.resource) args.resource = draft.resource;
    if (draft.reason) args.reason = draft.reason;
    return (
      <section className="sheet sheet--wide" aria-label="Подтверждение заявки">
        <DraftCard action={draft.type} args={args} text="Проверьте заявку: пока вы не нажали «Отправить», она никуда не уходит." />
        <div className="form__actions">
          <button type="button" className="btn btn--link" onClick={() => navigate('/actions')}>
            К моим заявкам
          </button>
        </div>
      </section>
    );
  }

  return (
    <section className="sheet sheet--wide new-request" aria-label="Новая заявка">
      <div className="filters" role="group" aria-label="Тип заявки">
        {tab('access', 'Доступ к системе')}
        {tab('absence', 'Отгул')}
      </div>
      <form className="request-form" onSubmit={submit} noValidate>
        {type === 'access' ? (
          <label className="field">
            <span>Система</span>
            <select value={resource} onChange={(e) => setResource(e.target.value)}>
              {SYSTEMS.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.label}
                </option>
              ))}
            </select>
          </label>
        ) : (
          <>
            <label className="field">
              <span>С</span>
              <input type="date" value={from} min={todayMsk()} onChange={(e) => setFrom(e.target.value)} required />
            </label>
            <label className="field">
              <span>По</span>
              <input type="date" value={to} min={from} onChange={(e) => setTo(e.target.value)} required />
            </label>
          </>
        )}
        <label className="field request-form__wide">
          <span>Причина {type === 'access' ? '(необязательно)' : ''}</span>
          <textarea value={reason} maxLength={MAX_REASON} rows={3} onChange={(e) => setReason(e.target.value)} />
        </label>
        <p className="form__hint request-form__wide">
          {type === 'access'
            ? 'Доступ согласует руководитель; после согласования шаг «Получить доступы» в плане закроется сам.'
            : 'Отгул согласует руководитель. Предупредите наставника — Бадди сам не пишет за вас.'}
        </p>
        {(invalid || create.isError) && (
          <p className="error request-form__wide" role="alert">
            {invalid ?? (create.error as Error).message}
          </p>
        )}
        <div className="form__actions request-form__wide">
          <button type="submit" className="btn btn--primary" disabled={create.isPending}>
            {create.isPending ? 'Готовлю черновик…' : 'Подготовить заявку'}
          </button>
          <Link to="/actions" className="btn btn--link">
            Отмена
          </Link>
        </div>
      </form>
    </section>
  );
}
