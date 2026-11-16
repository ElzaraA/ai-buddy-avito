import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { AgentResponseSchema, type StoredMessage } from '../../api/schemas';
import { AssistantMessage } from '../../components/AssistantMessage';
import { useChatStore } from '../../store/chat';

const when = (iso?: string) =>
  iso
    ? new Date(iso).toLocaleString('ru-RU', {
        timeZone: 'Europe/Moscow',
        day: 'numeric',
        month: 'long',
        hour: '2-digit',
        minute: '2-digit',
      })
    : '';

function StoredReply({ m }: { m: StoredMessage }) {
  if (m.status === 'failed') return <div className="msg msg--error">{m.content || 'Ответ не получен'}</div>;
  if (m.status !== 'done') return <div className="msg msg--pending">Ответ ещё готовится…</div>;
  try {
    const parsed = AgentResponseSchema.safeParse(JSON.parse(m.result ?? ''));
    if (parsed.success) return <AssistantMessage response={parsed.data} />;
  } catch {
    // старый формат — показываем текст
  }
  return <div className="msg msg--answer">{m.content}</div>;
}

/** Переписка одного диалога (только чтение): черновики заявок из истории не отправляются повторно. */
function Transcript({ id }: { id: string }) {
  const user = useChatStore((s) => s.user);
  const msgs = useQuery({ queryKey: ['session-messages', user, id], queryFn: () => api.sessionMessages(id) });
  if (msgs.isLoading) return <p className="muted">Загружаю переписку…</p>;
  if (msgs.isError)
    return (
      <p className="error" role="alert">
        Не удалось загрузить переписку.
      </p>
    );
  return (
    <div className="transcript">
      {msgs.data?.map((m) =>
        m.role === 'user' ? (
          <div key={m.id} className="row row--user">
            <div className="msg msg--user">{m.content}</div>
          </div>
        ) : (
          <div key={m.id} className="row transcript__reply">
            <StoredReply m={m} />
          </div>
        ),
      )}
    </div>
  );
}

/** История диалогов с Бадди: свои прошлые разговоры, новые сверху. */
export function HistoryView() {
  const user = useChatStore((s) => s.user);
  const [open, setOpen] = useState<string | null>(null);
  const sessions = useQuery({ queryKey: ['sessions', user], queryFn: api.sessions });

  return (
    <section className="sheet sheet--wide history" aria-label="История диалогов">
      {sessions.isLoading && <p className="muted">Загружаю историю…</p>}
      {sessions.isError && (
        <p className="error" role="alert">
          Не удалось загрузить историю.{' '}
          <button type="button" className="btn btn--link" onClick={() => void sessions.refetch()}>
            Повторить
          </button>
        </p>
      )}
      {sessions.data && sessions.data.length === 0 && (
        <p className="muted">Диалогов пока нет — задайте Бадди первый вопрос, и он появится здесь.</p>
      )}
      <ul className="history__list">
        {sessions.data?.map((s) => (
          <li key={s.id} className="history__item">
            <button
              type="button"
              className="history__head"
              aria-expanded={open === s.id}
              onClick={() => setOpen(open === s.id ? null : s.id)}
            >
              <span className="history__q">{s.first_question || 'Диалог без вопросов'}</span>
              <span className="history__meta">
                {when(s.last_at)} · сообщений: {s.messages}
              </span>
            </button>
            {open === s.id && <Transcript id={s.id} />}
          </li>
        ))}
      </ul>
    </section>
  );
}
