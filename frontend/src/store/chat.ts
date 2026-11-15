import { create } from 'zustand';
import type { AgentResponse } from '../api/schemas';

export type ChatMessage =
  | { id: string; role: 'user'; text: string }
  | { id: string; role: 'assistant'; status: 'pending'; startedAt: number }
  | { id: string; role: 'assistant'; status: 'done'; response: AgentResponse }
  | { id: string; role: 'assistant'; status: 'failed'; error: string }
  /** Напоминание о встрече: Бадди пишет его в чат сам, без вопроса стажёра. */
  | { id: string; role: 'reminder'; text: string };

type Outcome = { kind: 'done'; response: AgentResponse } | { kind: 'failed'; error: string };

export const DEMO_USERS = [
  { login: 'intern.backend', label: 'Стажёр Backend' },
  { login: 'intern.junior', label: 'Второй стажёр Backend' },
  { login: 'intern.other', label: 'Стажёр другой команды' },
  { login: 'mentor.backend', label: 'Наставник Backend' },
  { login: 'lead.backend', label: 'Руководитель Backend' },
  { login: 'lead.frontend', label: 'Руководитель Frontend' },
] as const;

interface ChatState {
  user: string;
  sessionId: string | null;
  messages: ChatMessage[];
  /** Ответы, пришедшие по SSE раньше, чем сообщение попало в список (быстрый агент). */
  early: Record<string, Outcome>;
  setUser: (login: string) => void;
  setSession: (id: string | null) => void;
  addUser: (id: string, text: string) => void;
  addPending: (id: string) => void;
  resolve: (id: string, response: AgentResponse) => void;
  fail: (id: string, error: string) => void;
  reset: () => void;
  /** Добавить напоминание о встрече (повтор с тем же id игнорируется). */
  addReminder: (id: string, text: string) => void;
  /** Напоминания, которые пришли, пока чат был закрыт: счётчик на пункте меню. */
  unreadReminders: number;
  readReminders: () => void;
  /**
   * Запрос из оболочки (боковое меню, карточки рабочего стола) к полю ввода чата:
   * подставить текст и поставить фокус. nonce меняется на каждый запрос, чтобы повтор сработал.
   */
  composer: { text: string; nonce: number } | null;
  prefill: (text: string) => void;
  /** Вопрос, который чат должен отправить сам (клик по подсказке вне чата). */
  queued: { text: string; nonce: number } | null;
  ask: (text: string) => void;
  takeQueued: () => void;
}

let nonce = 0;

/** Роль переживает перезагрузку вкладки (sessionStorage), новая вкладка начинает со стажёра. */
const USER_KEY = 'ai-buddy.user';
const DEFAULT_USER = 'intern.backend';
const isDemoUser = (login: string | null): login is string => DEMO_USERS.some((u) => u.login === login);

export function loadUser(): string {
  try {
    const saved = sessionStorage.getItem(USER_KEY);
    return isDemoUser(saved) ? saved : DEFAULT_USER;
  } catch {
    return DEFAULT_USER;
  }
}

function saveUser(login: string) {
  try {
    sessionStorage.setItem(USER_KEY, login);
  } catch {
    // приватный режим или запрет хранилища — роль просто не запомнится
  }
}

/**
 * Роль по логину, пока /me ещё грузится: сразу после переключения на наставника
 * не мелькает стол стажёра (и не уходят запросы стажёра, на которые сервер ответит 403).
 */
export const guessRole = (login: string) =>
  login.startsWith('lead.') ? 'lead' : login.startsWith('mentor.') ? 'buddy' : 'intern';

const toMessage = (id: string, o: Outcome): ChatMessage =>
  o.kind === 'done'
    ? { id, role: 'assistant', status: 'done', response: o.response }
    : { id, role: 'assistant', status: 'failed', error: o.error };

export const useChatStore = create<ChatState>((set) => {
  const settle = (id: string, outcome: Outcome) =>
    set((s) => {
      const target = s.messages.find((m) => m.id === id);
      if (!target) return { early: { ...s.early, [id]: outcome } };
      if (target.role !== 'assistant' || target.status !== 'pending') return {}; // уже завершено
      return { messages: s.messages.map((m) => (m.id === id ? toMessage(id, outcome) : m)) };
    });

  return {
    user: loadUser(),
    sessionId: null,
    messages: [],
    early: {},
    composer: null,
    queued: null,
    unreadReminders: 0,
    setUser: (login) => {
      saveUser(login);
      set({ user: login, sessionId: null, messages: [], early: {}, composer: null, queued: null, unreadReminders: 0 });
    },
    addReminder: (id, text) =>
      set((s) =>
        s.messages.some((m) => m.id === id)
          ? {}
          : { messages: [...s.messages, { id, role: 'reminder', text }], unreadReminders: s.unreadReminders + 1 },
      ),
    readReminders: () => set({ unreadReminders: 0 }),
    prefill: (text) => set({ composer: { text, nonce: ++nonce } }),
    ask: (text) => set({ queued: { text, nonce: ++nonce } }),
    takeQueued: () => set({ queued: null }),
    setSession: (id) => set({ sessionId: id }),
    addUser: (id, text) => set((s) => ({ messages: [...s.messages, { id, role: 'user', text }] })),
    addPending: (id) =>
      set((s) => {
        const outcome = s.early[id];
        if (outcome) {
          const { [id]: _, ...rest } = s.early;
          return { messages: [...s.messages, toMessage(id, outcome)], early: rest };
        }
        return {
          messages: [...s.messages, { id, role: 'assistant', status: 'pending', startedAt: Date.now() }],
        };
      }),
    resolve: (id, response) => settle(id, { kind: 'done', response }),
    fail: (id, error) => settle(id, { kind: 'failed', error }),
    reset: () => set({ sessionId: null, messages: [], early: {} }),
  };
});
