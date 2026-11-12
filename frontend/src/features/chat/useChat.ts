import { useCallback, useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, setDemoUser } from '../../api/client';
import { MessageDoneSchema, SseErrorSchema } from '../../api/schemas';
import { useChatStore } from '../../store/chat';

/** Сколько ждём ответ агента, прежде чем показать ошибку (на бэкенде run = 60 с). */
export const AGENT_TIMEOUT_MS = 65_000;

/**
 * Асинхронный чат: POST сообщения возвращает 202 сразу, ответ агента приходит
 * по SSE. Интерфейс не блокируется, пока работает LLM.
 */
export function useChat() {
  const qc = useQueryClient();
  const { user, sessionId, setSession, addUser, addPending, resolve, fail } = useChatStore();
  const sourceRef = useRef<EventSource | null>(null);
  const timers = useRef(new Map<string, ReturnType<typeof setTimeout>>());
  const [connectError, setConnectError] = useState<string | null>(null);

  const clearTimer = (id: string) => {
    const t = timers.current.get(id);
    if (t) clearTimeout(t);
    timers.current.delete(id);
  };

  const openStream = useCallback(
    (id: string) =>
      new Promise<void>((resolveOpen) => {
        sourceRef.current?.close();
        const es = new EventSource(api.eventsUrl(id));
        sourceRef.current = es;
        es.onopen = () => resolveOpen();
        es.addEventListener('message_done', (ev) => {
          const parsed = MessageDoneSchema.safeParse(JSON.parse((ev as MessageEvent).data));
          if (!parsed.success) return;
          const { message_id, status, result } = parsed.data;
          clearTimer(message_id);
          if (status === 'done' && result) {
            resolve(message_id, result);
            if (result.type === 'action') qc.invalidateQueries({ queryKey: ['progress'] });
          } else if (status !== 'done') {
            fail(message_id, 'Помощник не смог ответить. Попробуйте ещё раз.');
          }
        });
        es.addEventListener('error', (ev) => {
          const data = (ev as MessageEvent).data;
          if (typeof data !== 'string') return; // обрыв соединения — EventSource переподключится сам
          const parsed = SseErrorSchema.safeParse(JSON.parse(data));
          if (parsed.success) {
            clearTimer(parsed.data.message_id);
            fail(parsed.data.message_id, parsed.data.error);
          }
        });
        // не держим UI, если поток не открылся
        setTimeout(resolveOpen, 1500);
      }),
    [fail, qc, resolve],
  );

  const ensureSession = useCallback(async () => {
    if (sessionId) return sessionId;
    const s = await api.createSession();
    setSession(s.id);
    await openStream(s.id);
    return s.id;
  }, [openStream, sessionId, setSession]);

  const send = useCallback(
    async (text: string) => {
      const question = text.trim();
      if (!question) return;
      setConnectError(null);
      setDemoUser(user);
      try {
        const id = await ensureSession();
        const res = await api.sendMessage(id, question);
        addUser(res.user_message_id, question);
        addPending(res.assistant_message_id);
        timers.current.set(
          res.assistant_message_id,
          setTimeout(
            () => fail(res.assistant_message_id, 'Помощник не ответил вовремя. Попробуйте ещё раз.'),
            AGENT_TIMEOUT_MS,
          ),
        );
      } catch (e) {
        setConnectError(e instanceof Error ? e.message : 'Не удалось отправить сообщение');
      }
    },
    [addPending, addUser, ensureSession, fail, user],
  );

  // смена пользователя или размонтирование — закрываем поток и таймеры
  useEffect(() => {
    const t = timers.current;
    return () => {
      sourceRef.current?.close();
      sourceRef.current = null;
      t.forEach(clearTimeout);
      t.clear();
    };
  }, [user]);

  return { send, connectError };
}
