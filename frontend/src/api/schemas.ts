import { z } from 'zod';

// Схемы повторяют фактические ответы Go-бэкенда (ветка egor/ai-buddy-backend).
// Если бэкенд меняет формат — правим здесь, и TypeScript покажет все места.

export const UserSchema = z.object({
  id: z.string(),
  name: z.string(),
  role: z.string(),
  team_id: z.string(),
});
export type User = z.infer<typeof UserSchema>;

export const ChecklistItemSchema = z.object({
  id: z.string(),
  title: z.string(),
  stage: z.string(),
  done: z.boolean(),
  owner: z.string().optional(),
  /** todo | waiting | done (бэк v2); у старого бэка поля нет */
  status: z.string().optional(),
  related_action_id: z.string().optional(),
});
export type ChecklistItem = z.infer<typeof ChecklistItemSchema>;

export const ProgressSchema = z.object({
  items: z.array(ChecklistItemSchema).nullable().transform((v) => v ?? []),
  next_step: z.string().default(''),
  /** бэк v2 считает сам; у старого бэка поля нет — считаем на фронте */
  percent_done: z.number().optional(),
});
export type Progress = z.infer<typeof ProgressSchema>;

export const SessionSchema = z.object({
  id: z.string(),
  user_id: z.string().optional(),
  created_at: z.string().optional(),
});

/**
 * Ответ на отправку сообщения. Бэк v1 возвращает только { messageId } (id ответа ассистента),
 * бэк v2 — { user_message_id, assistant_message_id }. Приводим к одному виду.
 */
export const SendMessageResponseSchema = z
  .object({
    user_message_id: z.string().optional(),
    assistant_message_id: z.string().optional(),
    messageId: z.string().optional(),
    status: z.string().default('pending'),
  })
  .refine((r) => Boolean(r.assistant_message_id ?? r.messageId), {
    message: 'Сервер не вернул id сообщения',
  })
  .transform((r) => {
    const assistant = (r.assistant_message_id ?? r.messageId) as string;
    return {
      assistant_message_id: assistant,
      user_message_id: r.user_message_id ?? `${assistant}-user`,
      status: r.status,
    };
  });

export const SourceSchema = z.object({
  id: z.string(),
  source: z.string().optional(),
  stage: z.string().optional(),
});
export type Source = z.infer<typeof SourceSchema>;

export const ToolResultSchema = z.object({
  ok: z.boolean(),
  message: z.string().default(''),
  data: z.record(z.unknown()).optional(),
});
export type ToolResult = z.infer<typeof ToolResultSchema>;

/** Ответ агента: answer | action | draft | refusal. */
export const AgentResponseSchema = z.object({
  type: z.enum(['answer', 'action', 'draft', 'refusal']),
  text: z.string().optional(),
  action: z.string().optional(),
  args: z.record(z.string()).optional(),
  result: ToolResultSchema.optional(),
  sources: z.array(SourceSchema).optional(),
  buddy_name: z.string().optional(),
  manager_name: z.string().optional(),
});
export type AgentResponse = z.infer<typeof AgentResponseSchema>;

export const AbsenceSchema = z.object({
  id: z.string(),
  type: z.enum(['absence', 'access']).default('absence'),
  resource: z.string().optional(),
  date_from: z.string().default(''),
  date_to: z.string().default(''),
  reason: z.string().optional(),
  status: z.string(),
});
export type Absence = z.infer<typeof AbsenceSchema>;

export const ApiErrorSchema = z.object({
  error: z.object({ code: z.string(), message: z.string() }),
});

/** Бэкенд в разных версиях присылает messageId или message_id — принимаем оба. */
const withMessageId = <T extends z.ZodRawShape>(shape: T) =>
  z.preprocess((raw) => {
    if (raw && typeof raw === 'object' && !('message_id' in raw) && 'messageId' in raw) {
      return { ...(raw as Record<string, unknown>), message_id: (raw as { messageId: unknown }).messageId };
    }
    return raw;
  }, z.object({ message_id: z.string(), ...shape }));

/** Событие SSE message_done. */
export const MessageDoneSchema = withMessageId({
  status: z.string(),
  result: AgentResponseSchema.optional(),
});

export const SseErrorSchema = withMessageId({
  error: z.string(),
});
