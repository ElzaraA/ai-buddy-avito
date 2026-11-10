import { api, ApiError, getDemoUser, setDemoUser } from './client';
import { mockFetch } from '../test/utils';

afterEach(() => {
  vi.unstubAllGlobals();
  setDemoUser('intern.backend');
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
    await expect(api.completeStep('c9')).rejects.toMatchObject({ status: 403, code: 'forbidden' });
  });

  it('ошибка без JSON-тела — общий текст', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('oops', { status: 502 })));
    const err = await api.me().catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toContain('502');
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
    expect(JSON.parse(spy.mock.calls[1][1]?.body as string)).toEqual({ question: 'привет' });
    expect((await api.createAbsenceDraft({ date_from: '2026-11-20', date_to: '2026-11-20' })).status).toBe('draft');
    expect((await api.confirmAction('ABS-1')).status).toBe('pending');
    expect(api.eventsUrl('s1')).toBe('/api/v1/chat/sessions/s1/events');
  });
});
