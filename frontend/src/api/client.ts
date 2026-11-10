import { z } from 'zod';
import {
  AbsenceSchema,
  ApiErrorSchema,
  ProgressSchema,
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
  ) {
    super(message);
    this.name = 'ApiError';
  }
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
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      'X-Demo-User': demoUser,
      ...(init.headers ?? {}),
    },
  });
  const body: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    const parsed = ApiErrorSchema.safeParse(body);
    if (parsed.success) {
      throw new ApiError(res.status, parsed.data.error.code, parsed.data.error.message);
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
      body: JSON.stringify({ question }),
    }),
  createAbsenceDraft: (args: { date_from: string; date_to: string; reason?: string }) =>
    request('/api/v1/actions', AbsenceSchema, {
      method: 'POST',
      body: JSON.stringify({ type: 'absence', reason: 'personal', ...args }),
    }),
  confirmAction: (id: string) =>
    request(`/api/v1/actions/${encodeURIComponent(id)}/confirm`, AbsenceSchema, { method: 'POST' }),
  eventsUrl: (sessionId: string) => `${API_BASE}/api/v1/chat/sessions/${sessionId}/events`,
};
