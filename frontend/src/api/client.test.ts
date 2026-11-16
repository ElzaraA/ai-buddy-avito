import { api, ApiError, getDemoUser, NETWORK_ERROR_TEXT, setDemoUser, userMessage } from './client';
import { mockFetch } from '../test/utils';
import { AgentResponseSchema, DemoInfoSchema, KnowledgeGapsSchema, SignalsSchema } from './schemas';

afterEach(() => {
  vi.unstubAllGlobals();
  setDemoUser('intern.backend');
});

describe('схемы и фактические ответы бэкенда', () => {
  it('ответ агента с tool и sources[].title и пробел базы с last_at разбираются без потерь', () => {
    const action = AgentResponseSchema.parse({
      type: 'action',
      tool: 'checklist_get',
      result: { ok: true, message: '' },
      sources: [{ id: 'checklist.md#week1#2', source: 'checklist.md', stage: 'week1', title: 'Доступы' }],
      buddy_name: 'Анна Козлова',
    });
    expect(action.tool).toBe('checklist_get');
    expect(action.sources?.[0].title).toBe('Доступы');
    const gaps = KnowledgeGapsSchema.parse([
      { question: 'Сколько платят в Авито?', count: 1, last_at: '2026-11-14T10:47:48+03:00' },
    ]);
    expect(gaps[0].last_at).toBe('2026-11-14T10:47:48+03:00');
  });
});

describe('контракт: демо-режим и ответы наставника', () => {
  it('сигнал с ответом и без (старый бэк) разбирается; /demo — с безопасными значениями по умолчанию', () => {
    const [old, withReply] = SignalsSchema.parse([
      { id: 'SIG-1', text: 'VPN', topic: 'tech', done: false },
      { id: 'SIG-2', text: 'x', done: true, reply: 'ок', resolved_by: 'mentor-1', resolved_at: '2026-11-15T10:00:00Z' },
    ]);
    expect(old.reply).toBeUndefined();
    expect(withReply).toMatchObject({ reply: 'ок', resolved_by: 'mentor-1', resolved_at: '2026-11-15T10:00:00Z' });
    expect(DemoInfoSchema.parse({ demo_mode: true, llm: 'real' })).toEqual({ demo_mode: true, llm: 'real' });
    expect(DemoInfoSchema.parse({ llm: 'gpt' })).toEqual({ demo_mode: false, llm: 'mock' });
  });

  it('resolveSignal шлёт reply (обрезанный до 500) или пустое тело; mySignals, demo, resetDemo — по контракту', async () => {
    const spy = mockFetch([
      { method: 'POST', match: '/api/v1/team/signals/SIG-1/resolve', body: { ok: true } },
      { match: '/api/v1/signals/me', body: [{ id: 'SIG-1', text: 'VPN', done: true, reply: 'ок' }] },
      { match: '/api/v1/demo', body: { demo_mode: true, llm: 'mock' } },
      { method: 'POST', match: '/api/v1/demo/reset', body: { ok: true } },
    ]);
    await api.resolveSignal('SIG-1', `  ${'a'.repeat(600)} `);
    expect(JSON.parse(String(spy.mock.calls[0][1]?.body)).reply).toHaveLength(500);
    await api.resolveSignal('SIG-1');
    expect(JSON.parse(String(spy.mock.calls[1][1]?.body))).toEqual({});
    expect((await api.mySignals())[0].reply).toBe('ок');
    expect(await api.demo()).toEqual({ demo_mode: true, llm: 'mock' });
    expect((await api.resetDemo()).ok).toBe(true);
  });

  it('сброс при выключенном демо — 404 как ApiError', async () => {
    mockFetch([]);
    await expect(api.resetDemo()).rejects.toMatchObject({ status: 404 });
  });
});

describe('api client', () => {
  it('передаёт тестового пользователя в X-Demo-User и валидирует ответ', async () => {
    const spy = mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-2', name: 'Стажёр', role: 'intern', team_id: 'frontend' } },
    ]);
    setDemoUser('intern.other');
    expect(getDemoUser()).toBe('intern.other');
    const me = await api.me();
    expect(me.team_id).toBe('frontend');
    const headers = spy.mock.calls[0][1]?.headers as Record<string, string>;
    expect(headers['X-Demo-User']).toBe('intern.other');
  });

  it('прогресс: null вместо списка превращается в пустой массив', async () => {
    mockFetch([{ match: '/api/v1/onboarding/progress', body: { items: null, next_step: '' } }]);
    expect(await api.progress()).toEqual({ items: [], next_step: '' });
  });

  it('ошибка бэкенда в формате {error} превращается в ApiError', async () => {
    mockFetch([
      {
        method: 'PATCH',
        match: /steps\/c9$/,
        status: 403,
        body: { error: { code: 'forbidden', message: 'step belongs to another user' } },
      },
    ]);
    await expect(api.completeStep('c9')).rejects.toMatchObject({
      status: 403,
      code: 'forbidden',
      message: 'Недостаточно прав для этого действия',
      detail: 'step belongs to another user',
    });
  });

  it('пользователю — русский текст: словарь по коду, русское сообщение бэка как есть', () => {
    expect(userMessage(409, 'conflict', 'action is not pending')).toBe('Заявка уже обработана, список обновлён');
    expect(userMessage(404, 'not_found', 'action not found')).toBe('Не найдено, обновите страницу');
    expect(userMessage(429, 'rate_limited', 'Слишком много запросов, попробуйте через 3 с')).toBe(
      'Слишком много запросов, попробуйте через 3 с',
    );
    expect(userMessage(500, 'sse_unsupported', 'flusher not available')).toBe('Ошибка сервера (500)');
  });

  it('сервер недоступен (fetch бросает TypeError) — «Нет связи с сервером»', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('Failed to fetch');
      }),
    );
    const err = await api.me().catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 0, code: 'network', message: NETWORK_ERROR_TEXT, detail: 'Failed to fetch' });
  });

  it('teamSummary ходит в /team/summary и заполняет значения по умолчанию', async () => {
    mockFetch([{ match: '/api/v1/team/summary', body: { interns: 2, avg_percent_done: 50, actions: { pending: 1 } } }]);
    expect(await api.teamSummary()).toEqual({
      interns: 2,
      avg_percent_done: 50,
      steps_waiting: 0,
      actions: { pending: 1 },
      unanswered_topics: 0,
      unanswered_requests: 0,
      questions_closed: 0,
      signals_total: 0,
      refusals_total: 0,
      mentor_minutes_saved: 0,
      minutes_per_question: 5,
      closed_share: 0,
    });
  });

  it('«Пульс» и достижения: POST /pulse с оценкой, team/pulse с null вместо отметок', async () => {
    const spy = mockFetch([
      { method: 'POST', match: '/api/v1/pulse', body: { date: '2026-11-15', mood: 2 } },
      {
        match: '/api/v1/team/pulse',
        body: [{ user: { id: 'intern-1', name: 'С', role: 'intern', team_id: 'backend' }, today: null, avg7: null, trend: '?', risk: false, days: null }],
      },
      { match: '/api/v1/achievements', body: null },
    ]);
    expect((await api.submitPulse(2, 'устал')).mood).toBe(2);
    expect(JSON.parse(spy.mock.calls[0][1]?.body as string)).toEqual({ mood: 2, note: 'устал' });
    const [p] = await api.teamPulse();
    expect(p).toMatchObject({ today: null, avg7: null, trend: 'flat', days: [], risk_reason: '' });
    expect(await api.achievements()).toEqual([]);
  });

  it('ошибка без JSON-тела — общий текст', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('oops', { status: 502 })));
    const err = await api.me().catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toContain('502');
  });

  it('отправка сообщения: формат бэка v1 { messageId } приводится к общему виду', async () => {
    mockFetch([
      { method: 'POST', match: '/api/v1/chat/sessions/s1/messages', status: 202, body: { messageId: 'a7', status: 'pending' } },
    ]);
    expect(await api.sendMessage('s1', 'x')).toEqual({
      assistant_message_id: 'a7',
      user_message_id: 'a7-user',
      status: 'pending',
    });
  });

  it('отправка сообщения: ответ без id — понятная ошибка', async () => {
    mockFetch([{ method: 'POST', match: '/api/v1/chat/sessions/s1/messages', status: 202, body: { status: 'pending' } }]);
    await expect(api.sendMessage('s1', 'x')).rejects.toThrow('Сервер не вернул id сообщения');
  });

  it('чат и действия ходят в нужные эндпоинты', async () => {
    const spy = mockFetch([
      { method: 'POST', match: '/api/v1/chat/sessions', body: { id: 's1' } },
      {
        method: 'POST',
        match: '/api/v1/chat/sessions/s1/messages',
        status: 202,
        body: { user_message_id: 'u1', assistant_message_id: 'a1', status: 'pending' },
      },
      {
        method: 'POST',
        match: '/api/v1/actions',
        status: 201,
        body: { id: 'ABS-1', date_from: '2026-11-20', date_to: '2026-11-20', status: 'draft' },
      },
      {
        method: 'POST',
        match: '/api/v1/actions/ABS-1/confirm',
        body: { id: 'ABS-1', date_from: '2026-11-20', date_to: '2026-11-20', status: 'pending' },
      },
    ]);
    expect((await api.createSession()).id).toBe('s1');
    expect((await api.sendMessage('s1', 'привет')).assistant_message_id).toBe('a1');
    expect(JSON.parse(spy.mock.calls[1][1]?.body as string)).toEqual({ text: 'привет', question: 'привет' });
    expect((await api.createAbsenceDraft({ date_from: '2026-11-20', date_to: '2026-11-20' })).status).toBe('draft');
    expect((await api.confirmAction('ABS-1')).status).toBe('pending');
    expect(api.eventsUrl('s1')).toBe('/api/v1/chat/sessions/s1/events?user=intern.backend');
    setDemoUser('mentor.backend');
    expect(api.eventsUrl('s1')).toContain('?user=mentor.backend');
  });
});
