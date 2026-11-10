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
});
export type ChecklistItem = z.infer<typeof ChecklistItemSchema>;

export const ProgressSchema = z.object({
  items: z.array(ChecklistItemSchema).nullable().transform((v) => v ?? []),
  next_step: z.string().default(''),
});
export type Progress = z.infer<typeof ProgressSchema>;

export const SessionSchema = z.object({
  id: z.string(),
  user_id: z.string().optional(),
  created_at: z.string().optional(),
});

export const SendMessageResponseSchema = z.object({
  user_message_id: z.string(),
  assistant_message_id: z.string(),
  status: z.string(),
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
  date_from: z.string(),
  date_to: z.string(),
  reason: z.string().optional(),
  status: z.string(),
});
export type Absence = z.infer<typeof AbsenceSchema>;

export const ApiErrorSchema = z.object({
  error: z.object({ code: z.string(), message: z.string() }),
});

/** Событие SSE message_done. */
export const MessageDoneSchema = z.object({
  message_id: z.string(),
  status: z.string(),
  result: AgentResponseSchema.optional(),
});

export const SseErrorSchema = z.object({
  message_id: z.string(),
  error: z.string(),
});
