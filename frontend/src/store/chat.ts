import { create } from 'zustand';
import type { AgentResponse } from '../api/schemas';

export type ChatMessage =
  | { id: string; role: 'user'; text: string }
  | { id: string; role: 'assistant'; status: 'pending'; startedAt: number }
  | { id: string; role: 'assistant'; status: 'done'; response: AgentResponse }
  | { id: string; role: 'assistant'; status: 'failed'; error: string };

type Outcome = { kind: 'done'; response: AgentResponse } | { kind: 'failed'; error: string };

export const DEMO_USERS = [
  { login: 'intern.backend', label: 'Стажёр Backend' },
  { login: 'intern.other', label: 'Стажёр другой команды' },
  { login: 'mentor.backend', label: 'Наставник Backend' },
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
}

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
    user: 'intern.backend',
    sessionId: null,
    messages: [],
    early: {},
    setUser: (login) => set({ user: login, sessionId: null, messages: [], early: {} }),
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
