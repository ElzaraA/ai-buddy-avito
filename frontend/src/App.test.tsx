import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { App } from './App';
import { getDemoUser } from './api/client';
import { useChatStore } from './store/chat';
import { FakeEventSource, mockFetch, renderWithQuery } from './test/utils';

beforeEach(() => {
  vi.stubGlobal('EventSource', FakeEventSource);
  useChatStore.getState().setUser('intern.backend');
});
afterEach(() => vi.unstubAllGlobals());

describe('App', () => {
  it('показывает пользователя, чат и прогресс; переключение тестового пользователя', async () => {
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/onboarding/progress', body: { items: [], next_step: '' } },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByText('Стажёр Backend', { selector: '.me__name' })).toBeInTheDocument();
    expect(screen.getByText('команда backend')).toBeInTheDocument();
    expect(screen.getByLabelText('Чат с AI-buddy')).toBeInTheDocument();
    expect(await screen.findByText('Моя адаптация')).toBeInTheDocument();
    expect(screen.getByText(/не являются регламентами Авито/)).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText('Войти как'), 'intern.other');
    await waitFor(() => expect(getDemoUser()).toBe('intern.other'));
    expect(useChatStore.getState().user).toBe('intern.other');
  });

  it('в переключателе есть руководитель команды frontend', async () => {
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/onboarding/progress', body: { items: [], next_step: '' } },
    ]);
    renderWithQuery(<App />);
    expect(screen.getByRole('option', { name: 'Руководитель Frontend' })).toHaveValue('lead.frontend');
    await userEvent.selectOptions(screen.getByLabelText('Войти как'), 'lead.frontend');
    await waitFor(() => expect(getDemoUser()).toBe('lead.frontend'));
  });

  it('оболочка: «Сообщить наставнику» в меню подставляет заготовку в чат, карточка открывает план', async () => {
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/onboarding/progress', body: { items: [], next_step: 'Прочитать Playbook' } },
    ]);
    renderWithQuery(<App />);
    expect(screen.getByRole('button', { name: 'Рабочий стол' })).toHaveAttribute('aria-current', 'page');

    const menu = screen.getByRole('navigation', { name: 'Разделы' });
    await userEvent.click(within(menu).getByRole('button', { name: 'Сообщить наставнику' }));
    expect(screen.getByLabelText('Сообщение')).toHaveValue('Передай наставнику: ');
    expect(screen.getByLabelText('Сообщение')).toHaveFocus();

    await userEvent.click(await screen.findByRole('button', { name: /Следующий шаг: Прочитать Playbook/ }));
    const nav = screen.getByRole('navigation', { name: 'Разделы' });
    expect(within(nav).getByRole('button', { name: 'Мой план' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByLabelText('Чат с AI-buddy').closest('.chat-card')).not.toBeVisible();
  });

  it('навигация одна — боковое меню; пункт меню и заголовок раздела называются одинаково', async () => {
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/onboarding/progress', body: { items: [], next_step: '' } },
      { match: '/api/v1/actions', body: [] },
    ]);
    renderWithQuery(<App />);
    expect(screen.queryByRole('group', { name: 'Вкладки' })).not.toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: 'Разделы' });
    for (const name of ['Рабочий стол', 'Мой план', 'Мои заявки', 'Что умеет Бадди']) {
      await userEvent.click(within(nav).getByRole('button', { name }));
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(name);
      expect(screen.getAllByRole('button', { name })).toHaveLength(1);
    }
  });

  it('пока /me не ответил, наставнику не показывается стол стажёра (роль — по логину)', async () => {
    useChatStore.getState().setUser('mentor.backend');
    const spy = mockFetch([{ match: '/api/v1/team/progress', body: [] }]); // /me → 404
    renderWithQuery(<App />);
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Кабинет команды');
    expect(screen.queryByRole('button', { name: /Открыть план/ })).not.toBeInTheDocument();
    await waitFor(() => expect(spy).toHaveBeenCalled());
    // запросы стажёра (на них сервер ответил бы наставнику 403) не уходят
    expect(spy.mock.calls.some(([u]) => /\/pulse\/me|\/achievements|\/onboarding\/progress/.test(String(u)))).toBe(false);
  });

  it('руководителю вместо плана адаптации показывается кабинет команды', async () => {
    useChatStore.getState().setUser('lead.backend');
    mockFetch([
      { match: '/api/v1/me', body: { id: 'lead-1', name: 'Руководитель Backend', role: 'lead', team_id: 'backend' } },
      { match: '/api/v1/team/progress', body: [] },
      { match: '/api/v1/team/actions?status=pending', body: [] },
      { match: '/api/v1/team/knowledge-gaps', body: [] },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByText('Моя команда')).toBeInTheDocument();
    expect(screen.getByText(/руководитель: согласование заявок/)).toBeInTheDocument();
    expect(screen.queryByText('Моя адаптация')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Чат с AI-buddy')).toBeInTheDocument();
  });
});
