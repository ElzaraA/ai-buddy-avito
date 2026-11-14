import { useCallback, useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, ApiError, setDemoUser } from '../../api/client';
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
  const { user, setSession, addUser, addPending, resolve, fail } = useChatStore();
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

  // Создание сессии в полёте: параллельные отправки (двойной клик по подсказке) ждут один и тот же
  // промис, а не создают вторую сессию, закрывая поток первой.
  const creating = useRef<{ user: string; promise: Promise<string> } | null>(null);

  const ensureSession = useCallback((): Promise<string> => {
    if (creating.current?.user === user) return creating.current.promise;
    const current = useChatStore.getState().sessionId;
    if (current) return Promise.resolve(current);
    const promise = api.createSession().then(async (s) => {
      // пока создавалась сессия, сменили учётку — не привязываем чужую сессию к новому пользователю
      if (useChatStore.getState().user !== user) throw new Error('Учётка сменилась, отправьте сообщение ещё раз');
      setSession(s.id);
      await openStream(s.id);
      return s.id;
    });
    const settle = () => {
      if (creating.current?.promise === promise) creating.current = null;
    };
    promise.then(settle, settle);
    creating.current = { user, promise };
    return promise;
  }, [openStream, setSession, user]);

  const [sending, setSending] = useState(0);

  const send = useCallback(
    async (text: string) => {
      const question = text.trim();
      if (!question) return;
      setConnectError(null);
      setDemoUser(user);
      setSending((n) => n + 1);
      try {
        let id = await ensureSession();
        let res;
        try {
          res = await api.sendMessage(id, question);
        } catch (e) {
          // 429 session_full: диалог упёрся в лимит сообщений — продолжаем в новой сессии
          if (!(e instanceof ApiError && e.code === 'session_full')) throw e;
          setSession(null);
          id = await ensureSession();
          res = await api.sendMessage(id, question);
        }
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
      } finally {
        setSending((n) => n - 1);
      }
    },
    [addPending, addUser, ensureSession, fail, setSession, user],
  );

  // смена пользователя или размонтирование — закрываем поток и таймеры
  useEffect(() => {
    const t = timers.current;
    return () => {
      creating.current = null;
      sourceRef.current?.close();
      sourceRef.current = null;
      t.forEach(clearTimeout);
      t.clear();
    };
  }, [user]);

  return { send, connectError, sending: sending > 0 };
}
