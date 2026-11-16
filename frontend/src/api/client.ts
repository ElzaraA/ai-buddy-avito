import { z } from 'zod';
import {
  AbsenceSchema,
  AchievementsSchema,
  ActionListSchema,
  MyPulseSchema,
  PulseEntrySchema,
  TeamPulseSchema,
  ApiErrorSchema,
  OccurrenceSchema,
  RemindersSchema,
  ScheduleSchema,
  DemoInfoSchema,
  InternCardSchema,
  KnowledgeDocSchema,
  KnowledgeListSchema,
  KnowledgeSearchSchema,
  RecommendationsSchema,
  SessionsSchema,
  StoredMessagesSchema,
  TaskSchema,
  TasksSchema,
  KnowledgeGapsSchema,
  OkSchema,
  ProgressSchema,
  SignalsSchema,
  TeamProgressSchema,
  TeamSummarySchema,
  SendMessageResponseSchema,
  SessionSchema,
  UserSchema,
} from './schemas';

/** Явный API-клиент (требование кейса при отдельном размещении статики). */

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    /** Исходный текст ошибки от сервера или браузера — для отладки, не для показа пользователю. */
    public detail: string = message,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

/** Понятные пользователю тексты по коду ошибки бэкенда (сам бэкенд часто отвечает по-английски). */
const ERROR_TEXTS: Record<string, string> = {
  conflict: 'Заявка уже обработана, список обновлён',
  forbidden: 'Недостаточно прав для этого действия',
  not_found: 'Не найдено, обновите страницу',
  unauthorized: 'Тестовая учётка не найдена, выберите пользователя заново',
  bad_request: 'Проверьте введённые данные и попробуйте ещё раз',
};

export const NETWORK_ERROR_TEXT = 'Нет связи с сервером, попробуйте ещё раз';

/**
 * Текст ошибки для интерфейса. Русское сообщение бэкенда (например, rate_limited) показываем как есть,
 * английские технические — заменяем словарём по коду.
 */
export function userMessage(status: number, code: string, serverMessage: string): string {
  if (/[а-яё]/i.test(serverMessage)) return serverMessage;
  return ERROR_TEXTS[code] ?? `Ошибка сервера (${status})`;
}

export const API_BASE: string = import.meta.env.VITE_API_URL ?? '';

let demoUser = 'intern.backend';
/** Тестовая учётка. Авторизация в Авито уже есть — в прототипе её имитирует заголовок X-Demo-User. */
export function setDemoUser(login: string) {
  demoUser = login;
}
export function getDemoUser() {
  return demoUser;
}

async function request<S extends z.ZodTypeAny>(
  path: string,
  schema: S,
  init: RequestInit = {},
): Promise<z.output<S>> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, {
      ...init,
      headers: {
        'Content-Type': 'application/json',
        'X-Demo-User': demoUser,
        ...(init.headers ?? {}),
      },
    });
  } catch (e) {
    // fetch бросает TypeError («Failed to fetch»), если сервер недоступен
    throw new ApiError(0, 'network', NETWORK_ERROR_TEXT, e instanceof Error ? e.message : String(e));
  }
  const body: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    const parsed = ApiErrorSchema.safeParse(body);
    if (parsed.success) {
      const { code, message } = parsed.data.error;
      throw new ApiError(res.status, code, userMessage(res.status, code, message), message);
    }
    throw new ApiError(res.status, 'http_error', `Ошибка сервера (${res.status})`);
  }
  return schema.parse(body);
}

export const api = {
  me: () => request('/api/v1/me', UserSchema),
  progress: () => request('/api/v1/onboarding/progress', ProgressSchema),
  completeStep: (id: string) =>
    request(`/api/v1/onboarding/steps/${encodeURIComponent(id)}`, ProgressSchema, {
      method: 'PATCH',
      body: JSON.stringify({ status: 'done' }),
    }),
  createSession: () => request('/api/v1/chat/sessions', SessionSchema, { method: 'POST' }),
  sendMessage: (sessionId: string, question: string) =>
    request(`/api/v1/chat/sessions/${sessionId}/messages`, SendMessageResponseSchema, {
      method: 'POST',
      // text — по контракту openapi, question — для старых версий бэка
      body: JSON.stringify({ text: question, question }),
    }),
  createAbsenceDraft: (args: { date_from: string; date_to: string; reason?: string }) =>
    request('/api/v1/actions', AbsenceSchema, {
      method: 'POST',
      body: JSON.stringify({ type: 'absence', reason: 'personal', ...args }),
    }),
  confirmAction: (id: string) =>
    request(`/api/v1/actions/${encodeURIComponent(id)}/confirm`, AbsenceSchema, { method: 'POST' }),
  /** Мои заявки (черновики и отправленные), новые сверху. */
  listActions: () => request('/api/v1/actions', ActionListSchema),
  /** Заявка на доступ из формы — черновик; уходит руководителю только после confirmAction. */
  createAccessDraft: (args: { resource: string; reason?: string }) =>
    request('/api/v1/actions', AbsenceSchema, { method: 'POST', body: JSON.stringify({ type: 'access', ...args }) }),
  /** Задачи адаптации своей команды и рекомендации следующей. */
  tasks: () => request('/api/v1/tasks', TasksSchema),
  task: (id: string) => request(`/api/v1/tasks/${encodeURIComponent(id)}`, TaskSchema),
  moveTask: (id: string, op: 'start' | 'complete' | 'skip' | 'reopen') =>
    request(`/api/v1/tasks/${encodeURIComponent(id)}/${op}`, TaskSchema, { method: 'POST' }),
  recommendations: (limit = 3) => request(`/api/v1/recommendations?limit=${limit}`, RecommendationsSchema),
  /** База знаний: статьи, поиск по разделам и статья — с правами по роли и команде. */
  knowledge: () => request('/api/v1/knowledge', KnowledgeListSchema),
  searchKnowledge: (q: string) => request(`/api/v1/knowledge?q=${encodeURIComponent(q)}`, KnowledgeSearchSchema),
  article: (id: string) => request(`/api/v1/knowledge/${encodeURIComponent(id)}`, KnowledgeDocSchema),
  /** История диалогов и сообщения одного диалога. */
  sessions: () => request('/api/v1/chat/sessions', SessionsSchema),
  sessionMessages: (id: string) =>
    request(`/api/v1/chat/sessions/${encodeURIComponent(id)}/messages`, StoredMessagesSchema),
  /** Карточка стажёра своей команды (наставник, руководитель). */
  teamIntern: (id: string) => request(`/api/v1/team/interns/${encodeURIComponent(id)}`, InternCardSchema),
  /** Расписание: встречи команды и личные созвоны на days дней вперёд. */
  schedule: (days = 7) => request(`/api/v1/schedule?days=${days}`, ScheduleSchema),
  createMeeting: (m: { title: string; start: string; duration_min: number; with?: string }) =>
    request('/api/v1/schedule', OccurrenceSchema, { method: 'POST', body: JSON.stringify(m) }),
  cancelMeeting: (meetingId: string) =>
    request(`/api/v1/schedule/${encodeURIComponent(meetingId)}/cancel`, z.unknown(), { method: 'POST' }),
  /** Встречи, о которых пора напомнить в чате; после показа — ackReminder, чтобы не повторялись. */
  reminders: () => request('/api/v1/reminders', RemindersSchema),
  ackReminder: (id: string) =>
    request(`/api/v1/reminders/${encodeURIComponent(id)}/ack`, z.unknown(), { method: 'POST' }),
  cancelAction: (id: string) =>
    request(`/api/v1/actions/${encodeURIComponent(id)}/cancel`, AbsenceSchema, { method: 'POST' }),
  // Кабинет наставника и руководителя — только своя команда
  teamProgress: () => request('/api/v1/team/progress', TeamProgressSchema),
  teamPending: () => request('/api/v1/team/actions?status=pending', ActionListSchema),
  knowledgeGaps: () => request('/api/v1/team/knowledge-gaps', KnowledgeGapsSchema),
  /** Сообщения стажёров наставнику «что-то не так». */
  teamSignals: () => request('/api/v1/team/signals', SignalsSchema),
  /** Закрыть сигнал; reply (≤500 символов) — необязательный ответ стажёру. */
  resolveSignal: (id: string, reply = '') => {
    const text = reply.trim().slice(0, 500);
    return request(`/api/v1/team/signals/${encodeURIComponent(id)}/resolve`, z.unknown(), {
      method: 'POST',
      body: JSON.stringify(text ? { reply: text } : {}),
    });
  },
  /** Обращения стажёра к наставнику и ответы на них. */
  mySignals: () => request('/api/v1/signals/me', SignalsSchema),
  /** Демо-режим и какая модель отвечает (404 — у старого бэка ручки нет). */
  demo: () => request('/api/v1/demo', DemoInfoSchema),
  /** Сброс демо-данных к исходному состоянию (404, если демо-режим выключен). */
  resetDemo: () => request('/api/v1/demo/reset', OkSchema, { method: 'POST' }),
  /** Сводка «метрик пользы» по команде. */
  teamSummary: () => request('/api/v1/team/summary', TeamSummarySchema),
  /** «Пульс команды» — самочувствие стажёров своей команды. */
  teamPulse: () => request('/api/v1/team/pulse', TeamPulseSchema),
  /** «Пульс» стажёра: отметка за сегодня (повторная — перезаписывает). */
  submitPulse: (mood: number, note = '') =>
    request('/api/v1/pulse', PulseEntrySchema, { method: 'POST', body: JSON.stringify({ mood, note }) }),
  myPulse: () => request('/api/v1/pulse/me', MyPulseSchema),
  achievements: () => request('/api/v1/achievements', AchievementsSchema),
  decideAction: (id: string, decision: 'approve' | 'reject', comment = '') =>
    request(`/api/v1/actions/${encodeURIComponent(id)}/${decision}`, AbsenceSchema, {
      method: 'POST',
      body: JSON.stringify({ comment }),
    }),
  // EventSource не умеет слать заголовки, поэтому учётка передаётся в ?user=
  eventsUrl: (sessionId: string) =>
    `${API_BASE}/api/v1/chat/sessions/${sessionId}/events?user=${encodeURIComponent(demoUser)}`,
};
