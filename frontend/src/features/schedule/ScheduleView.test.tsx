import { renderHook, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ReactNode } from 'react';
import { dayTitle, groupByDay, nextHourLocal, ScheduleView } from './ScheduleView';
import { useReminders } from './useReminders';
import { mockFetch, renderWithQuery } from '../../test/utils';
import { useChatStore } from '../../store/chat';

afterEach(() => vi.unstubAllGlobals());
beforeEach(() => useChatStore.setState({ messages: [], unreadReminders: 0 }));

const standup = {
  id: 'standup@2026-11-16T11:00',
  title: 'Стендап команды',
  kind: 'standup',
  start: '2026-11-16T11:00:00+03:00',
  end: '2026-11-16T11:15:00+03:00',
  place: 'Яндекс Телемост',
  recurring: true,
};
const personal = {
  id: 'MTG-000001@2026-11-16T15:00',
  title: 'Разбор задачи',
  kind: 'personal',
  start: '2026-11-16T15:00:00+03:00',
  end: '2026-11-16T15:30:00+03:00',
  with: 'Иван Петров',
  meeting_id: 'MTG-000001',
  recurring: false,
};
const oneOnOne = {
  ...standup,
  id: 'one@18',
  title: '1:1 с наставником',
  kind: 'one_on_one',
  start: '2026-11-18T16:00:00+03:00',
  end: '2026-11-18T16:30:00+03:00',
};

describe('schedule helpers', () => {
  const now = new Date('2026-11-16T08:00:00Z'); // 11:00 МСК, понедельник
  it('подписи дней: сегодня, завтра, дальше — с заглавной', () => {
    expect(dayTitle('2026-11-16', now)).toBe('Сегодня, понедельник, 16 ноября');
    expect(dayTitle('2026-11-17', now)).toBe('Завтра, вторник, 17 ноября');
    expect(dayTitle('2026-11-18', now)).toBe('Среда, 18 ноября');
  });
  it('группирует встречи по дням по Москве', () => {
    const late = { ...standup, id: 'late', start: '2026-11-16T22:30:00Z' }; // 01:30 МСК следующего дня
    const groups = groupByDay([standup, personal, late]);
    expect(groups.map((g) => [g.day, g.items.length])).toEqual([
      ['2026-11-16', 2],
      ['2026-11-17', 1],
    ]);
  });
  it('время по умолчанию — следующий полный час по Москве', () => {
    expect(nextHourLocal(new Date('2026-11-16T08:20:00Z'))).toBe('2026-11-16T12:00');
    expect(nextHourLocal(new Date('2026-11-16T20:30:00Z'))).toBe('2026-11-17T00:00');
  });
});

describe('ScheduleView', () => {
  it('показывает встречи по дням; свой созвон можно отменить', async () => {
    const fetch = mockFetch([
      {
        match: /\/api\/v1\/schedule\?days=7$/,
        body: {
          from: '',
          to: '',
          reminder_lead_min: 10,
          items: [standup, personal, oneOnOne],
        },
      },
      {
        method: 'POST',
        match: '/api/v1/schedule/MTG-000001/cancel',
        body: null,
      },
    ]);
    renderWithQuery(<ScheduleView />);
    expect(await screen.findByText('Разбор задачи')).toBeInTheDocument();
    expect(screen.getByText(/за 10 минут/)).toBeInTheDocument();
    expect(screen.getByText('Иван Петров')).toBeInTheDocument();
    expect(screen.getByText('Мой созвон')).toBeInTheDocument();
    expect(screen.getByText('1:1')).toBeInTheDocument();
    // командные встречи отменить нельзя — кнопка только у своего созвона
    expect(screen.getAllByRole('button', { name: /Отменить созвон/ })).toHaveLength(1);

    await userEvent.click(screen.getByRole('button', { name: 'Отменить созвон «Разбор задачи»' }));
    await waitFor(() =>
      expect(fetch.mock.calls.some(([u, i]) => String(u).endsWith('/MTG-000001/cancel') && i?.method === 'POST')).toBe(
        true,
      ),
    );
  });

  it('добавляет созвон: отправляет форму и очищает название', async () => {
    const fetch = mockFetch([
      {
        match: /\/api\/v1\/schedule\?days=7$/,
        body: { from: '', to: '', items: [] },
      },
      {
        method: 'POST',
        match: '/api/v1/schedule',
        status: 201,
        body: personal,
      },
    ]);
    renderWithQuery(<ScheduleView />);
    expect(await screen.findByText('На ближайшую неделю встреч нет.')).toBeInTheDocument();
    const submit = screen.getByRole('button', {
      name: 'Добавить в расписание',
    });
    expect(submit).toBeDisabled();

    await userEvent.type(screen.getByLabelText('Название'), 'Разбор задачи');
    await userEvent.selectOptions(screen.getByLabelText('Длительность'), '45');
    await userEvent.type(screen.getByLabelText('С кем'), 'Иван Петров');
    await userEvent.click(submit);

    await waitFor(() => expect(screen.getByLabelText('Название')).toHaveValue(''));
    const post = fetch.mock.calls.find(([u, i]) => String(u).endsWith('/api/v1/schedule') && i?.method === 'POST');
    expect(JSON.parse(String(post?.[1]?.body))).toMatchObject({
      title: 'Разбор задачи',
      duration_min: 45,
      with: 'Иван Петров',
    });
  });

  it('ошибка сервера при добавлении показывается у формы', async () => {
    mockFetch([
      {
        match: /\/api\/v1\/schedule\?days=7$/,
        body: { from: '', to: '', items: [] },
      },
      {
        method: 'POST',
        match: '/api/v1/schedule',
        status: 400,
        body: {
          error: {
            code: 'invalid_request',
            message: 'Время встречи уже прошло',
          },
        },
      },
    ]);
    renderWithQuery(<ScheduleView />);
    await userEvent.type(await screen.findByLabelText('Название'), 'Поздно');
    await userEvent.click(screen.getByRole('button', { name: 'Добавить в расписание' }));
    expect(await screen.findByRole('alert')).toBeInTheDocument();
  });

  it('не загрузилось — предлагает повторить', async () => {
    mockFetch([{ match: /\/api\/v1\/schedule\?days=7$/, status: 500, body: {} }]);
    renderWithQuery(<ScheduleView />);
    expect(await screen.findByText(/Не удалось загрузить расписание/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument();
  });
});

describe('useReminders', () => {
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      {children}
    </QueryClientProvider>
  );

  it('пишет напоминание в чат один раз и подтверждает показ', async () => {
    const fetch = mockFetch([
      {
        match: '/api/v1/reminders',
        body: [{ ...standup, text: 'Через 15 минут — Стендап команды в 11:00.' }],
      },
      {
        method: 'POST',
        match: `/api/v1/reminders/${encodeURIComponent(standup.id)}/ack`,
        body: null,
      },
    ]);
    renderHook(() => useReminders(), { wrapper });
    await waitFor(() => expect(useChatStore.getState().messages).toHaveLength(1));
    expect(useChatStore.getState().messages[0]).toEqual({
      id: standup.id,
      role: 'reminder',
      text: 'Через 15 минут — Стендап команды в 11:00.',
    });
    expect(useChatStore.getState().unreadReminders).toBe(1);
    await waitFor(() => expect(fetch.mock.calls.some(([u]) => String(u).includes('/ack'))).toBe(true));

    // то же напоминание повторно в чат не попадает
    useChatStore.getState().addReminder(standup.id, 'повтор');
    expect(useChatStore.getState().messages).toHaveLength(1);
    useChatStore.getState().readReminders();
    expect(useChatStore.getState().unreadReminders).toBe(0);
  });
});
