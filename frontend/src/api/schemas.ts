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
  /** access — шаг закрывается сам после согласования заявки на доступ */
  kind: z.string().optional(),
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
  /** заголовок раздела базы знаний */
  title: z.string().optional(),
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
  /** имя выполненного инструмента для type=action */
  tool: z.string().optional(),
  result: ToolResultSchema.optional(),
  sources: z.array(SourceSchema).optional(),
  buddy_name: z.string().optional(),
  manager_name: z.string().optional(),
  /** ответ дан резервной моделью: основная LLM недоступна */
  degraded: z.boolean().optional(),
  /** готовые следующие реплики стажёра — кнопки под ответом */
  suggestions: z.array(z.string()).nullable().optional(),
});
export type AgentResponse = z.infer<typeof AgentResponseSchema>;

/** GET /api/v1/team/signals — сообщения стажёров наставнику «что-то не так». */
export const SignalSchema = z.object({
  id: z.string(),
  user_id: z.string().default(''),
  text: z.string().default(''),
  topic: z.string().default('other'),
  at: z.string().optional(),
  done: z.boolean().default(false),
  /** ответ наставника стажёру (необязателен; у старого бэка поля нет) */
  reply: z.string().optional(),
  resolved_by: z.string().optional(),
  resolved_at: z.string().optional(),
});
export type Signal = z.infer<typeof SignalSchema>;
/** GET /api/v1/team/signals (наставник) и GET /api/v1/signals/me (стажёр — свои обращения). */
export const SignalsSchema = z.array(SignalSchema).nullable().transform((v) => v ?? []);

/** GET /api/v1/demo — честно показываем, что это демо и какая модель отвечает. */
export const DemoInfoSchema = z.object({
  demo_mode: z.boolean().default(false),
  llm: z.enum(['mock', 'real']).catch('mock'),
});
export type DemoInfo = z.infer<typeof DemoInfoSchema>;

/** POST /api/v1/demo/reset → { ok: true } */
export const OkSchema = z.object({ ok: z.boolean().default(true) }).passthrough();

/** Запись истории заявки: кто и когда перевёл её в статус. */
export const ActionEventSchema = z.object({
  at: z.string().optional(),
  by: z.string().default(''),
  status: z.string(),
  comment: z.string().optional(),
});

export const AbsenceSchema = z.object({
  id: z.string(),
  type: z.enum(['absence', 'access']).default('absence'),
  resource: z.string().optional(),
  user_id: z.string().optional(),
  date_from: z.string().default(''),
  date_to: z.string().default(''),
  reason: z.string().optional(),
  /** draft | pending | approved | rejected | cancelled */
  status: z.string(),
  history: z.array(ActionEventSchema).optional(),
});
export type Absence = z.infer<typeof AbsenceSchema>;
export const ActionListSchema = z.array(AbsenceSchema).nullable().transform((v) => v ?? []);

/** Кабинет наставника и руководителя (GET /api/v1/team/*). */
export const InternProgressSchema = z.object({
  user: UserSchema,
  items: z.array(ChecklistItemSchema).nullable().transform((v) => v ?? []),
  next_step: z.string().default(''),
  percent_done: z.number().default(0),
});
export type InternProgress = z.infer<typeof InternProgressSchema>;
export const TeamProgressSchema = z.array(InternProgressSchema).nullable().transform((v) => v ?? []);

/** GET /api/v1/team/summary — «метрики пользы» для руководителя. */
export const TeamSummarySchema = z.object({
  interns: z.number().default(0),
  avg_percent_done: z.number().default(0),
  steps_waiting: z.number().default(0),
  actions: z
    .record(z.number())
    .nullable()
    .optional()
    .transform((v) => v ?? {}),
  unanswered_topics: z.number().default(0),
  unanswered_requests: z.number().default(0),
  /** «Сэкономлено наставнику»: вопросы стажёров, закрытые Бадди без человека */
  questions_closed: z.number().default(0),
  /** сообщения стажёров, переданные наставнику */
  signals_total: z.number().default(0),
  refusals_total: z.number().default(0),
  mentor_minutes_saved: z.number().default(0),
  /** допущение оценки: минут наставника на один типовой вопрос */
  minutes_per_question: z.number().default(5),
  /** доля закрытых Бадди: closed / (closed + signals + refusals), 0..1 */
  closed_share: z.number().default(0),
});
export type TeamSummary = z.infer<typeof TeamSummarySchema>;

/** «Пульс»: отметка самочувствия стажёра за день (1 — тяжело … 5 — отлично). */
export const PulseEntrySchema = z.object({
  date: z.string(),
  mood: z.number().int().min(1).max(5),
  note: z.string().optional(),
  at: z.string().optional(),
});
export type PulseEntry = z.infer<typeof PulseEntrySchema>;

/** GET /api/v1/pulse/me — последние 14 отметок (старые первыми) и сегодняшняя дата сервера. */
export const MyPulseSchema = z.object({
  today: z.string(),
  entries: z.array(PulseEntrySchema).nullable().transform((v) => v ?? []),
});
export type MyPulse = z.infer<typeof MyPulseSchema>;

/** GET /api/v1/team/pulse — «Пульс команды»: сначала стажёры, которым нужно внимание. */
export const InternPulseSchema = z.object({
  user: UserSchema,
  today: z.number().nullable().default(null),
  avg7: z.number().nullable().default(null),
  trend: z.enum(['up', 'down', 'flat']).catch('flat'),
  risk: z.boolean().default(false),
  risk_reason: z.string().default(''),
  days: z
    .array(z.object({ date: z.string(), mood: z.number().nullable().default(null) }))
    .nullable()
    .transform((v) => v ?? []),
  note: z.string().optional(),
  note_date: z.string().optional(),
});
export type InternPulse = z.infer<typeof InternPulseSchema>;
export const TeamPulseSchema = z.array(InternPulseSchema).nullable().transform((v) => v ?? []);

/** GET /api/v1/achievements — достижения стажёра в порядке от простых к сложным. */
export const AchievementSchema = z.object({
  id: z.string(),
  title: z.string(),
  description: z.string().default(''),
  unlocked: z.boolean(),
  unlocked_at: z.string().optional(),
  hint: z.string().default(''),
});
export type Achievement = z.infer<typeof AchievementSchema>;
export const AchievementsSchema = z.array(AchievementSchema).nullable().transform((v) => v ?? []);

export const KnowledgeGapSchema = z.object({
  question: z.string(),
  count: z.number(),
  last_at: z.string().optional(),
});
export const KnowledgeGapsSchema = z.array(KnowledgeGapSchema).nullable().transform((v) => v ?? []);

/** Встреча в расписании: ритуал команды или личный созвон. */
export const OccurrenceSchema = z.object({
  id: z.string(),
  title: z.string(),
  kind: z.string(),
  start: z.string(),
  end: z.string(),
  place: z.string().optional(),
  with: z.string().optional(),
  recurring: z.boolean().default(false),
  meeting_id: z.string().optional(),
});
export type Occurrence = z.infer<typeof OccurrenceSchema>;

export const ScheduleSchema = z.object({
  from: z.string(),
  to: z.string(),
  reminder_lead_min: z.number().default(15),
  items: z.array(OccurrenceSchema).nullable().transform((v) => v ?? []),
});
export type Schedule = z.infer<typeof ScheduleSchema>;

/** Напоминание о встрече, которое Бадди присылает в чат. */
export const ReminderSchema = OccurrenceSchema.extend({ text: z.string() });
export type Reminder = z.infer<typeof ReminderSchema>;
export const RemindersSchema = z.array(ReminderSchema).nullable().transform((v) => v ?? []);

/** Задача адаптации (GET /api/v1/tasks). status — у стажёра; у наставника всегда todo. */
export const TaskSchema = z.object({
  id: z.string(),
  team: z.string().default(''),
  title: z.string(),
  summary: z.string().default(''),
  steps: z.array(z.string()).nullable().transform((v) => v ?? []).default([]),
  definition_of_done: z.string().default(''),
  difficulty: z.number().default(1),
  tags: z.array(z.string()).nullable().transform((v) => v ?? []).default([]),
  popularity: z.number().default(0),
  estimate_hours: z.number().default(0),
  knowledge: z.string().optional(),
  status: z.enum(['todo', 'in_progress', 'done', 'skipped']).catch('todo'),
  started_at: z.string().optional(),
  done_at: z.string().optional(),
});
export type Task = z.infer<typeof TaskSchema>;
export const TasksSchema = z.array(TaskSchema).nullable().transform((v) => v ?? []);

/** GET /api/v1/recommendations — следующие задачи от модели (или правила, если модели нет). */
export const RecommendationSchema = TaskSchema.extend({
  score: z.number().default(0),
  reasons: z.array(z.string()).nullable().transform((v) => v ?? []).default([]),
});
export type Recommendation = z.infer<typeof RecommendationSchema>;
export const RecommendationsSchema = z.object({
  source: z.enum(['model', 'rule_based']).catch('rule_based'),
  model: z.string().optional(),
  items: z.array(RecommendationSchema).nullable().transform((v) => v ?? []),
});
export type Recommendations = z.infer<typeof RecommendationsSchema>;

/** База знаний (GET /api/v1/knowledge…): только разделы, видимые роли и команде. */
export const KnowledgeSectionSchema = z.object({ title: z.string().default(''), stage: z.string().optional(), text: z.string() });
export const KnowledgeDocSchema = z.object({
  id: z.string(),
  title: z.string(),
  stages: z.array(z.string()).nullable().transform((v) => v ?? []).default([]),
  disclaimer: z.string().default(''),
  sections: z.array(KnowledgeSectionSchema).nullable().transform((v) => v ?? []).default([]),
});
export type KnowledgeDoc = z.infer<typeof KnowledgeDocSchema>;
export const KnowledgeListSchema = z.object({
  documents: z.array(KnowledgeDocSchema).nullable().transform((v) => v ?? []),
});
export const KnowledgeHitSchema = z.object({
  doc_id: z.string(),
  title: z.string().default(''),
  section: z.string().default(''),
  snippet: z.string().default(''),
});
export type KnowledgeHit = z.infer<typeof KnowledgeHitSchema>;
export const KnowledgeSearchSchema = z.object({
  query: z.string().default(''),
  hits: z.array(KnowledgeHitSchema).nullable().transform((v) => v ?? []),
});

/** История диалогов (GET /api/v1/chat/sessions) и сообщения диалога. */
export const SessionSummarySchema = z.object({
  id: z.string(),
  created_at: z.string().optional(),
  last_at: z.string().optional(),
  first_question: z.string().default(''),
  messages: z.number().default(0),
});
export type SessionSummary = z.infer<typeof SessionSummarySchema>;
export const SessionsSchema = z.array(SessionSummarySchema).nullable().transform((v) => v ?? []);
export const StoredMessageSchema = z.object({
  id: z.string(),
  role: z.string(),
  content: z.string().default(''),
  status: z.string().default('done'),
  result: z.string().optional(),
  created_at: z.string().optional(),
});
export type StoredMessage = z.infer<typeof StoredMessageSchema>;
export const StoredMessagesSchema = z.array(StoredMessageSchema).nullable().transform((v) => v ?? []);

/** GET /api/v1/team/interns/{id} — карточка стажёра для наставника и руководителя. */
export const InternCardSchema = z.object({
  user: UserSchema,
  progress: z.object({
    items: z.array(ChecklistItemSchema).nullable().transform((v) => v ?? []),
    next_step: z.string().default(''),
    percent_done: z.number().default(0),
  }),
  pulse: InternPulseSchema.omit({ user: true }),
  actions: ActionListSchema,
  signals: SignalsSchema,
  tasks: z
    .object({
      summary: z.object({
        total: z.number().default(0),
        in_progress: z.number().default(0),
        done: z.number().default(0),
        skipped: z.number().default(0),
      }),
      active: TasksSchema,
      recommendations: RecommendationsSchema,
    })
    .optional(),
});
export type InternCard = z.infer<typeof InternCardSchema>;

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
