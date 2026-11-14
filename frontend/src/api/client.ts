import { z } from 'zod';
import {
  AbsenceSchema,
  ActionListSchema,
  ApiErrorSchema,
  KnowledgeGapsSchema,
  ProgressSchema,
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
  cancelAction: (id: string) =>
    request(`/api/v1/actions/${encodeURIComponent(id)}/cancel`, AbsenceSchema, { method: 'POST' }),
  // Кабинет наставника и руководителя — только своя команда
  teamProgress: () => request('/api/v1/team/progress', TeamProgressSchema),
  teamPending: () => request('/api/v1/team/actions?status=pending', ActionListSchema),
  knowledgeGaps: () => request('/api/v1/team/knowledge-gaps', KnowledgeGapsSchema),
  /** Сводка «метрик пользы» по команде. */
  teamSummary: () => request('/api/v1/team/summary', TeamSummarySchema),
  decideAction: (id: string, decision: 'approve' | 'reject', comment = '') =>
    request(`/api/v1/actions/${encodeURIComponent(id)}/${decision}`, AbsenceSchema, {
      method: 'POST',
      body: JSON.stringify({ comment }),
    }),
  // EventSource не умеет слать заголовки, поэтому учётка передаётся в ?user=
  eventsUrl: (sessionId: string) =>
    `${API_BASE}/api/v1/chat/sessions/${sessionId}/events?user=${encodeURIComponent(demoUser)}`,
};
