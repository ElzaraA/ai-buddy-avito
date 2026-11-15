import { FormEvent, useEffect, useRef, useState } from 'react';
import { AssistantMessage } from '../../components/AssistantMessage';
import { useChatStore } from '../../store/chat';
import { useChat } from './useChat';

/** Подсказки сгруппированы по задачам стажёра: узнать правило, оформить заявку, понять следующий шаг. */
export const SUGGESTION_GROUPS: { title: string; hint: string; items: string[] }[] = [
  {
    title: 'Мой план',
    hint: 'что делать прямо сейчас',
    items: ['Какой следующий шаг по чек-листу?'],
  },
  {
    title: 'Оформить заявку',
    hint: 'помощник заполнит её за вас',
    items: ['Хочу взять отгул в пятницу', 'Запроси доступ в GitLab'],
  },
  {
    title: 'Узнать, как принято в команде',
    hint: 'ответ со ссылкой на документ',
    items: ['Как оформить отгул?', 'Как у нас устроено код-ревью?'],
  },
];

export const SUGGESTIONS = SUGGESTION_GROUPS.flatMap((g) => g.items);

/** Быстрые вопросы под перепиской: стажёру не нужно придумывать, что спросить дальше. */
export const FOLLOW_UPS = ['Что мне делать дальше?', 'Как оформить отгул?', 'Запроси доступ в GitLab'];

export const PROBLEM_PREFIX = 'Передай наставнику: ';

/** staff — наставник или руководитель: подсказки и вход «сообщить наставнику» нужны только стажёру. */
export function ChatPanel({ staff = false }: { staff?: boolean }) {
  const messages = useChatStore((s) => s.messages);
  const { send, connectError, sending } = useChat();
  const [text, setText] = useState('');
  const endRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Подсказки следующего шага присылает сервер вместе с ответом; если их нет — общий набор.
  const last = [...messages].reverse().find((m) => m.role !== 'user');
  const fromServer = last && last.status === 'done' ? (last.response.suggestions ?? []) : [];
  const followUps = fromServer.length > 0 ? fromServer : FOLLOW_UPS;

  // «Что-то не так?» — готовим сообщение наставнику: стажёру остаётся дописать, что случилось.
  const reportProblem = () => {
    setText(PROBLEM_PREFIX);
    inputRef.current?.focus();
  };

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
        {messages.length === 0 && staff && (
          <div className="empty">
            <h1 className="empty__title">Кабинет команды — справа</h1>
            <p className="empty__text">
              Там стажёры, которые просят помощи, заявки на согласование и вопросы, которых нет в базе знаний. В чате
              можно проверить, что помощник отвечает стажёрам.
            </p>
          </div>
        )}
        {messages.length === 0 && !staff && (
          <div className="empty">
            <h1 className="empty__title">Спросите здесь — до того, как писать наставнику</h1>
            <p className="empty__text">
              Помощник знает правила вашей команды и отвечает со ссылкой на документ. Заявки на отгул и доступы он
              заполнит сам — вам останется проверить и нажать «Отправить». Если ответа в базе нет, помощник скажет,
              к кому обратиться.
            </p>
            <div className="suggestion-groups">
              {SUGGESTION_GROUPS.map((g) => (
                <div key={g.title} className="suggestion-group">
                  <div className="suggestion-group__title">
                    {g.title} <span className="muted">— {g.hint}</span>
                  </div>
                  <div className="suggestions">
                    {g.items.map((s) => (
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
                </div>
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

      {messages.length > 0 && !staff && (
        <div className="followups" aria-label="Что можно сделать дальше">
          {followUps.map((s) => (
            <button key={s} className="suggestion suggestion--small" onClick={() => void send(s)} disabled={sending}>
              {s}
            </button>
          ))}
        </div>
      )}

      {!staff && (
        <div className="help-row">
          <button type="button" className="btn btn--link help-row__btn" onClick={reportProblem}>
            Что-то не так? Сообщить наставнику
          </button>
          <span className="muted">— это нормально, помощник передаст ваше сообщение</span>
        </div>
      )}

      <form className="composer" onSubmit={submit}>
        <input
          ref={inputRef}
          aria-label="Сообщение"
          placeholder="Спросите своими словами"
          value={text}
          onChange={(e) => setText(e.target.value)}
          maxLength={4000}
        />
        <button
          type="submit"
          className="btn btn--primary"
          disabled={!text.trim() || text.trim() === PROBLEM_PREFIX.trim()}
        >
          Отправить
        </button>
      </form>
    </section>
  );
}
