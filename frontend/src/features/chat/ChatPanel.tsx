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
  const { send, connectError } = useChat();
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
    <section className="panel chat" aria-label="Чат с AI-buddy">
      <header className="panel__header">
        <h2>AI-buddy</h2>
        <span className="muted">отвечает по базе знаний команды и помогает с заявками</span>
      </header>

      <div className="chat__messages" aria-live="polite">
        {messages.length === 0 && (
          <div className="empty">
            <p>Привет! Я помогу освоиться в команде. Например:</p>
            <div className="suggestions">
              {SUGGESTIONS.map((s) => (
                <button key={s} className="chip chip--button" onClick={() => void send(s)}>
                  {s}
                </button>
              ))}
            </div>
          </div>
        )}

        {messages.map((m) => {
          if (m.role === 'user') {
            return (
              <div key={m.id} className="row row--user">
                <div className="bubble bubble--user">{m.text}</div>
              </div>
            );
          }
          return (
            <div key={m.id} className="row">
              {m.status === 'pending' && (
                <div className="bubble bubble--assistant typing" role="status">
                  Помощник думает<span className="dots" />
                </div>
              )}
              {m.status === 'failed' && (
                <div className="card card--error" role="alert">
                  ⚠️ {m.error}
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

      <form className="chat__form" onSubmit={submit}>
        <input
          aria-label="Сообщение"
          placeholder="Спросите что-нибудь о команде, процессах или доступах…"
          value={text}
          onChange={(e) => setText(e.target.value)}
          maxLength={4000}
        />
        <button type="submit" disabled={!text.trim()}>
          Отправить
        </button>
      </form>
    </section>
  );
}
