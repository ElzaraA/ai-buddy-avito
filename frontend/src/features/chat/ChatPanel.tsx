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
    hint: 'Бадди заполнит её за вас',
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

/** В поле только заготовка «Передай наставнику:» — стажёр ещё ничего не дописал. */
export const isBarePrefix = (t: string) => t.trim() === PROBLEM_PREFIX.trim();

export const SLOW_SEND_MS = 4000;
export const SLOW_SEND_TEXT = 'Отправляю вопрос… Сервер просыпается после простоя — это может занять до минуты.';

/** staff — наставник или руководитель: подсказки и вход «сообщить наставнику» нужны только стажёру. */
export function ChatPanel({ staff = false }: { staff?: boolean }) {
  const messages = useChatStore((s) => s.messages);
  const { send, connectError, sending } = useChat();
  const [text, setText] = useState('');
  const listRef = useRef<HTMLDivElement>(null);
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
  // Заготовка без продолжения не должна «запирать» поле: другое действие (подсказка, карточка стола)
  // её заменяет. Свой текст стажёра не трогаем.
  const dropBarePrefix = () => setText((t) => (isBarePrefix(t) ? '' : t));
  const sendSuggestion = (s: string) => {
    dropBarePrefix();
    void send(s);
  };
  const clear = () => {
    setText('');
    inputRef.current?.focus();
  };

  // Запросы из оболочки: «Сообщить наставнику» в меню подставляет текст, подсказка — отправляет вопрос.
  const composer = useChatStore((s) => s.composer);
  const queued = useChatStore((s) => s.queued);
  const takeQueued = useChatStore((s) => s.takeQueued);

  useEffect(() => {
    if (!composer) return;
    if (composer.text) setText(composer.text);
    else setText((t) => (isBarePrefix(t) ? '' : t)); // «Написать Бадди» после «Сообщить наставнику»
    inputRef.current?.focus();
  }, [composer]);

  useEffect(() => {
    if (!queued) return;
    takeQueued();
    setText((t) => (isBarePrefix(t) ? '' : t));
    void send(queued.text);
  }, [queued, send, takeQueued]);

  // Прокручиваем только ленту сообщений, а не страницу: scrollIntoView двигал всю рабочую область
  // (на открытии — к низу пустого чата, при переходах — к полю ввода). Пустой чат не трогаем.
  useEffect(() => {
    const list = listRef.current;
    if (!list || messages.length === 0) return;
    list.scrollTop = list.scrollHeight;
  }, [messages]);

  // сервер не принял вопрос за SLOW_SEND_MS — скорее всего, бесплатный хостинг просыпается
  const [slow, setSlow] = useState(false);
  useEffect(() => {
    if (!sending) {
      setSlow(false);
      return;
    }
    const t = setTimeout(() => setSlow(true), SLOW_SEND_MS);
    return () => clearTimeout(t);
  }, [sending]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const value = text;
    setText(''); // поле очищается сразу, не дожидаясь сервера
    void send(value);
  };

  return (
    <section className="chat" aria-label="Чат с AI-buddy">
      <div className="chat__messages" ref={listRef} aria-live="polite">
        {messages.length === 0 && staff && (
          <div className="empty">
            <h2 className="empty__title">Проверьте, как Бадди отвечает стажёрам</h2>
            <p className="empty__text">
              Задайте вопрос так, как спросил бы стажёр. Просьбы о помощи, заявки на согласование и вопросы без ответа
              собраны в разделе «Кабинет команды».
            </p>
          </div>
        )}
        {messages.length === 0 && !staff && (
          <div className="empty">
            <h2 className="empty__title">Спросите здесь — до того, как писать наставнику</h2>
            <p className="empty__text">
              Бадди знает правила вашей команды и отвечает со ссылкой на документ. Заявку на отгул или доступ он
              заполнит сам — вам останется проверить и отправить. Если ответа в базе нет, Бадди скажет, к кому
              обратиться.
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
                        onClick={() => sendSuggestion(s)}
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
                  Бадди думает<span className="dots" aria-hidden />
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
      </div>

      {/* вопрос ещё не принят сервером (поле уже очищено) — показываем, что он в пути, и при холодном старте */}
      {sending && (
        <p className="muted chat__sending" role="status">
          {slow ? SLOW_SEND_TEXT : 'Отправляю вопрос…'}
        </p>
      )}

      {connectError && (
        <div className="error" role="alert">
          {connectError}
        </div>
      )}

      {messages.length > 0 && !staff && (
        <div className="followups" aria-label="Что можно сделать дальше">
          {followUps.map((s) => (
            <button key={s} className="suggestion suggestion--small" onClick={() => sendSuggestion(s)} disabled={sending}>
              {s}
            </button>
          ))}
        </div>
      )}

      {!staff && (
        <div className="help-row">
          <span className="muted">Застряли?</span>
          <button type="button" className="btn btn--link help-row__btn" onClick={reportProblem}>
            Сообщить наставнику
          </button>
        </div>
      )}

      <form className="composer" onSubmit={submit}>
        <input
          ref={inputRef}
          aria-label="Сообщение"
          placeholder={staff ? 'Вопрос от имени стажёра' : 'Спросите Бадди'}
          value={text}
          onChange={(e) => setText(e.target.value)}
          maxLength={4000}
        />
        {text && (
          <button type="button" className="composer__clear" onClick={clear} aria-label="Очистить поле" title="Очистить">
            ×
          </button>
        )}
        <button type="submit" className="btn btn--primary" disabled={!text.trim() || isBarePrefix(text)}>
          Отправить
        </button>
      </form>
    </section>
  );
}
