import { useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { useChatStore } from '../../store/chat';

/** Как часто спрашиваем сервер, не пора ли о чём-то напомнить. */
export const REMINDERS_POLL_MS = 30_000;

/**
 * Напоминания о встречах: сервер отдаёт встречи, до которых осталось меньше reminder_lead_min минут,
 * Бадди пишет их в чат, а мы подтверждаем показ — повторно то же напоминание не придёт.
 * Хук живёт в оболочке приложения, поэтому напоминание приходит, даже если открыт другой раздел.
 */
export function useReminders() {
  const user = useChatStore((s) => s.user);
  const addReminder = useChatStore((s) => s.addReminder);
  const { data } = useQuery({
    queryKey: ['reminders', user],
    queryFn: api.reminders,
    refetchInterval: REMINDERS_POLL_MS,
    // напоминание — не критичный запрос: при сбое просто спросим в следующий раз
    retry: false,
  });

  useEffect(() => {
    for (const r of data ?? []) {
      addReminder(r.id, r.text);
      // если подтверждение не дошло, напоминание придёт снова, но в чат второй раз не попадёт (дедуп по id)
      api.ackReminder(r.id).catch(() => undefined);
    }
  }, [data, addReminder]);
}
