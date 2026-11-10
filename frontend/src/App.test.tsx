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
});
