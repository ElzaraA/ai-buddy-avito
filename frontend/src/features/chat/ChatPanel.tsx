import { FormEvent, useEffect, useRef, useState } from 'react';
import { AssistantMessage } from '../../components/AssistantMessage';
import { useChatStore } from '../../store/chat';
import { useChat } from './useChat';

export const SUGGESTIONS = [
  'Как оформить отгул?',
  'Хочу взять отгул в пятницу',
  'Какой следующий шаг по чек-листу?',
  'Запроси доступ в GitLab',
  'Как у нас устроено код-ревью?',
];

export function ChatPanel() {
  const messages = useChatStore((s) => s.messages);
  const { send, connectError, sending } = useChat();
  const [text, setText] = useState('');
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    endRef.current?.scrollIntoView?.({ behavior: 'smooth' });
  }, [messages]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const value = text;
    setText(''); // поле очищается сразу, не дожидаясь сервера
    void send(value);
  };

  return (
    <section className="chat" aria-label="Чат с AI-buddy">
      <div className="chat__messages" aria-live="polite">
        {messages.length === 0 && (
          <div className="empty">
            <h1 className="empty__title">Спросите о команде, процессах и доступах</h1>
            <p className="empty__text">
              Помощник отвечает по базе знаний вашей команды и готовит заявки на отгул и доступы. Заявка уходит
              только после вашего подтверждения.
            </p>
            <div className="suggestions">
              {SUGGESTIONS.map((s) => (
                <button
                  key={s}
                  className="suggestion"
                  onClick={() => void send(s)}
                  // повторный клик, пока уходит первый вопрос, не отправляет его второй раз
                  disabled={sending}
                >
                  {s}
                </button>
              ))}
            </div>
            {sending && (
              <p className="muted" role="status">
                Отправляю вопрос…
              </p>
            )}
          </div>
        )}

        {messages.map((m) => {
          if (m.role === 'user') {
            return (
              <div key={m.id} className="row row--user">
                <div className="msg msg--user">{m.text}</div>
              </div>
            );
          }
          return (
            <div key={m.id} className="row">
              {m.status === 'pending' && (
                <div className="msg msg--pending" role="status">
                  Помощник думает<span className="dots" aria-hidden />
                </div>
              )}
              {m.status === 'failed' && (
                <div className="msg msg--error" role="alert">
                  {m.error}
                </div>
              )}
              {m.status === 'done' && <AssistantMessage response={m.response} />}
            </div>
          );
        })}
        <div ref={endRef} />
      </div>

      {connectError && (
        <div className="error" role="alert">
          {connectError}
        </div>
      )}

      <form className="composer" onSubmit={submit}>
        <input
          aria-label="Сообщение"
          placeholder="Например: где смотреть логи сервиса?"
          value={text}
          onChange={(e) => setText(e.target.value)}
          maxLength={4000}
        />
        <button type="submit" className="btn btn--primary" disabled={!text.trim()}>
          Отправить
        </button>
      </form>
    </section>
  );
}
