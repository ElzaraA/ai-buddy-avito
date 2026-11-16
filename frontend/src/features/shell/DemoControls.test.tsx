import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DemoControls, demoBadgeText, RESET_CONFIRM_TEXT } from './DemoControls';
import { App } from '../../App';
import { useChatStore } from '../../store/chat';
import { FakeEventSource, mockFetch, renderWithQuery } from '../../test/utils';

beforeEach(() => {
  vi.stubGlobal('EventSource', FakeEventSource);
  useChatStore.getState().setUser('intern.backend');
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('demoBadgeText', () => {
  it('честно называет модель', () => {
    expect(demoBadgeText({ demo_mode: true, llm: 'mock' })).toBe('Демо-режим · модель: правила (mock)');
    expect(demoBadgeText({ demo_mode: true, llm: 'real' })).toBe('Демо-режим · модель: LLM');
    expect(demoBadgeText({ demo_mode: false, llm: 'real' })).toBe('модель: LLM');
  });
});

describe('DemoControls', () => {
  it('бейдж с пояснением; без демо-режима кнопки сброса нет', async () => {
    mockFetch([{ match: '/api/v1/demo', body: { demo_mode: false, llm: 'real' } }]);
    renderWithQuery(<DemoControls onReset={() => {}} />);
    const badge = await screen.findByText('модель: LLM');
    expect(badge.closest('summary')).toHaveAttribute('title', expect.stringMatching(/языковая модель/));
    expect(screen.getByText(/языковая модель/, { selector: '.demo__text' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Сбросить демо' })).not.toBeInTheDocument();
  });

  it('старый бэк без /demo (404) — ничего не показываем', async () => {
    const spy = mockFetch([]);
    const { container } = renderWithQuery(<DemoControls onReset={() => {}} />);
    await waitFor(() => expect(spy).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });

  it('отказ в подтверждении — сброс не вызывается', async () => {
    const spy = mockFetch([{ match: '/api/v1/demo', body: { demo_mode: true, llm: 'mock' } }]);
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
    renderWithQuery(<DemoControls onReset={() => {}} />);
    await userEvent.click(await screen.findByRole('button', { name: 'Сбросить демо' }));
    expect(confirm).toHaveBeenCalledWith(RESET_CONFIRM_TEXT);
    expect(spy.mock.calls.some(([u]) => String(u).endsWith('/demo/reset'))).toBe(false);
  });

  it('404 на сброс — понятная ошибка', async () => {
    mockFetch([{ match: '/api/v1/demo', body: { demo_mode: true, llm: 'mock' } }]);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const onReset = vi.fn();
    renderWithQuery(<DemoControls onReset={onReset} />);
    await userEvent.click(await screen.findByRole('button', { name: 'Сбросить демо' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('демо-режим на сервере выключен');
    expect(onReset).not.toHaveBeenCalled();
  });

  it('в App: сброс очищает чат, перечитывает все данные и возвращает на стартовый раздел', async () => {
    const spy = mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/onboarding/progress', body: { items: [], next_step: '' } },
      { match: '/api/v1/actions', body: [] },
      { match: '/api/v1/demo', body: { demo_mode: true, llm: 'mock' } },
      { method: 'POST', match: '/api/v1/demo/reset', body: { ok: true } },
    ]);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const { client } = renderWithQuery(<App />);
    const invalidate = vi.spyOn(client, 'invalidateQueries');
    expect(await screen.findByText('Демо-режим · модель: правила (mock)')).toBeInTheDocument();

    useChatStore.getState().addUser('u1', 'старый вопрос');
    const nav = screen.getByRole('navigation', { name: 'Разделы' });
    await userEvent.click(within(nav).getByRole('button', { name: 'Мои заявки' }));
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Мои заявки');

    await userEvent.click(screen.getByRole('button', { name: 'Сбросить демо' }));
    await waitFor(() => expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Рабочий стол'));
    expect(spy.mock.calls.some(([u, init]) => String(u).endsWith('/demo/reset') && init?.method === 'POST')).toBe(true);
    expect(useChatStore.getState().messages).toHaveLength(0);
    expect(useChatStore.getState().sessionId).toBeNull();
    expect(invalidate).toHaveBeenCalledWith();
    expect(screen.queryByText('старый вопрос')).not.toBeInTheDocument();
  });
});
