import { FormEvent, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { Occurrence } from '../../api/schemas';
import { useChatStore } from '../../store/chat';

/** Расписание перечитываем раз в минуту: новые созвоны и закончившиеся встречи обновляются сами. */
export const SCHEDULE_REFRESH_MS = 60_000;

const TZ = 'Europe/Moscow';

const KIND_TONE: Record<string, string> = {
  standup: 'green',
  one_on_one: 'blue',
  planning: 'peach',
  retro: 'peach',
  personal: 'pink',
};

const KIND_LABEL: Record<string, string> = {
  standup: 'Команда',
  one_on_one: '1:1',
  planning: 'Спринт',
  retro: 'Спринт',
  personal: 'Мой созвон',
};

const dayKey = (iso: string) => new Date(iso).toLocaleDateString('sv-SE', { timeZone: TZ }); // 2026-11-16
const time = (iso: string) =>
  new Date(iso).toLocaleTimeString('ru-RU', {
    timeZone: TZ,
    hour: '2-digit',
    minute: '2-digit',
  });

/** «Сегодня, понедельник 16 ноября» / «Завтра, …» / «среда 18 ноября». */
export function dayTitle(key: string, now = new Date()) {
  const date = new Date(`${key}T12:00:00+03:00`);
  const label = date.toLocaleDateString('ru-RU', {
    timeZone: TZ,
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  });
  const today = dayKey(now.toISOString());
  const tomorrow = dayKey(new Date(now.getTime() + 24 * 3600 * 1000).toISOString());
  if (key === today) return `Сегодня, ${label}`;
  if (key === tomorrow) return `Завтра, ${label}`;
  return label.charAt(0).toUpperCase() + label.slice(1);
}

export function groupByDay(items: Occurrence[]) {
  const groups = new Map<string, Occurrence[]>();
  for (const o of items) {
    const k = dayKey(o.start);
    groups.set(k, [...(groups.get(k) ?? []), o]);
  }
  return Array.from(groups, ([day, list]) => ({ day, items: list }));
}

/** Ближайший полный час по Москве в формате поля datetime-local: «2026-11-16T15:00». */
export function nextHourLocal(now = new Date()) {
  const next = new Date(now.getTime() + 60 * 60 * 1000);
  const parts = new Intl.DateTimeFormat('sv-SE', {
    timeZone: TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    hourCycle: 'h23',
  }).format(next); // «2026-11-16 15»
  return `${parts.replace(' ', 'T')}:00`;
}

function MeetingCard({ o, onCancel, busy }: { o: Occurrence; onCancel: (id: string) => void; busy: boolean }) {
  const tone = KIND_TONE[o.kind] ?? 'green';
  return (
    <li className={`meeting meeting--${tone}`}>
      <div className="meeting__time">
        <span className="meeting__start">{time(o.start)}</span>
        <span className="meeting__end">до {time(o.end)}</span>
      </div>
      <div className="meeting__body">
        <span className="chip chip--light">{KIND_LABEL[o.kind] ?? 'Встреча'}</span>
        <span className="meeting__title">{o.title}</span>
        {(o.with || o.place) && <span className="meeting__meta">{[o.with, o.place].filter(Boolean).join(', ')}</span>}
      </div>
      {o.meeting_id && (
        <button
          type="button"
          className="btn btn--link meeting__cancel"
          onClick={() => onCancel(o.meeting_id!)}
          disabled={busy}
          aria-label={`Отменить созвон «${o.title}»`}
        >
          Отменить
        </button>
      )}
    </li>
  );
}

/** Раздел «Расписание»: встречи команды и свои созвоны; Бадди напоминает о них в чате. */
export function ScheduleView() {
  const user = useChatStore((s) => s.user);
  const qc = useQueryClient();
  const schedule = useQuery({
    queryKey: ['schedule', user],
    queryFn: () => api.schedule(7),
    refetchInterval: SCHEDULE_REFRESH_MS,
  });

  const [title, setTitle] = useState('');
  const [start, setStart] = useState(() => nextHourLocal());
  const [duration, setDuration] = useState(30);
  const [withWhom, setWithWhom] = useState('');

  const refresh = () => qc.invalidateQueries({ queryKey: ['schedule', user] });
  const create = useMutation({
    mutationFn: () =>
      api.createMeeting({
        title: title.trim(),
        start,
        duration_min: duration,
        with: withWhom.trim(),
      }),
    onSuccess: () => {
      setTitle('');
      setWithWhom('');
      refresh();
    },
  });
  const cancel = useMutation({
    mutationFn: api.cancelMeeting,
    onSuccess: refresh,
  });

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (title.trim()) create.mutate();
  };

  const lead = schedule.data?.reminder_lead_min ?? 15;
  const days = groupByDay(schedule.data?.items ?? []);

  return (
    <section className="sheet sheet--wide schedule" aria-label="Раздел «Расписание»">
      <p className="schedule__note">Бадди напомнит в чате за {lead} минут до начала каждой встречи.</p>

      {schedule.isLoading && <p className="muted">Загружаю расписание…</p>}
      {schedule.isError && (
        <div className="error" role="alert">
          Не удалось загрузить расписание.{' '}
          <button type="button" className="btn btn--link" onClick={() => void schedule.refetch()}>
            Повторить
          </button>
        </div>
      )}
      {schedule.data && days.length === 0 && <p className="muted">На ближайшую неделю встреч нет.</p>}

      <div className="schedule__days">
        {days.map((d) => (
          <section key={d.day} className="day" aria-label={dayTitle(d.day)}>
            <h3 className="day__title">{dayTitle(d.day)}</h3>
            <ul className="day__list">
              {d.items.map((o) => (
                <MeetingCard key={o.id} o={o} onCancel={(id) => cancel.mutate(id)} busy={cancel.isPending} />
              ))}
            </ul>
          </section>
        ))}
      </div>
      {cancel.isError && (
        <div className="error" role="alert">
          {(cancel.error as Error).message}
        </div>
      )}

      <form className="meeting-form" onSubmit={submit} aria-label="Добавить созвон">
        <h3 className="meeting-form__title">Добавить созвон</h3>
        <label className="field">
          <span>Название</span>
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Например: разбор первой задачи"
            maxLength={120}
            required
          />
        </label>
        <label className="field">
          <span>Начало</span>
          <input type="datetime-local" value={start} onChange={(e) => setStart(e.target.value)} required />
        </label>
        <label className="field">
          <span>Длительность</span>
          <select value={duration} onChange={(e) => setDuration(Number(e.target.value))}>
            {[15, 30, 45, 60, 90].map((m) => (
              <option key={m} value={m}>
                {m} минут
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span>С кем</span>
          <input
            value={withWhom}
            onChange={(e) => setWithWhom(e.target.value)}
            placeholder="Необязательно"
            maxLength={120}
          />
        </label>
        {create.isError && (
          <div className="error" role="alert">
            {(create.error as Error).message}
          </div>
        )}
        <button type="submit" className="pill-btn meeting-form__submit" disabled={!title.trim() || create.isPending}>
          {create.isPending ? 'Добавляю…' : 'Добавить в расписание'}
        </button>
      </form>
    </section>
  );
}
