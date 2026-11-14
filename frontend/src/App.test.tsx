import { screen, waitFor } from '@testing-library/react';
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
    expect(await screen.findByText('Стажёр Backend · backend')).toBeInTheDocument();
    expect(screen.getByLabelText('Чат с AI-buddy')).toBeInTheDocument();
    expect(await screen.findByText('Моя адаптация')).toBeInTheDocument();
    expect(screen.getByText(/не являются регламентами Авито/)).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText('Тестовый пользователь'), 'intern.other');
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
    await userEvent.selectOptions(screen.getByLabelText('Тестовый пользователь'), 'lead.frontend');
    await waitFor(() => expect(getDemoUser()).toBe('lead.frontend'));
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
