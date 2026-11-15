import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PulseStrip, TOLD_TEXT } from './PulseStrip';
import { useChatStore } from '../../store/chat';
import { mockFetch, renderWithQuery } from '../../test/utils';

beforeEach(() => useChatStore.getState().setUser('intern.backend'));
afterEach(() => vi.unstubAllGlobals());

const today = '2026-11-15';

describe('PulseStrip', () => {
  it('пять подписанных кнопок; выбор «Тяжело» — тёплый ответ и два выхода: наставник и Бадди', async () => {
    const spy = mockFetch([
      { match: '/api/v1/pulse/me', body: { today, entries: [] } },
      { method: 'POST', match: '/api/v1/pulse', body: { date: today, mood: 1 } },
    ]);
    renderWithQuery(<PulseStrip />);
    const group = await screen.findByRole('group', { name: 'Как вы сегодня?' });
    for (const name of ['Тяжело', 'Так себе', 'Нормально', 'Хорошо', 'Отлично']) {
      expect(screen.getByRole('button', { name })).toHaveAttribute('aria-pressed', 'false');
    }
    expect(group).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Тяжело' }));
    await waitFor(() => {
      const call = spy.mock.calls.find(([u, init]) => String(u).endsWith('/api/v1/pulse') && init?.method === 'POST');
      expect(JSON.parse(call![1]?.body as string)).toEqual({ mood: 1, note: '' });
    });
    expect(await screen.findByText(/Спасибо, что честно\. Так бывает — давайте разгрузим день/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Тяжело' })).toHaveAttribute('aria-pressed', 'true');

    await userEvent.click(screen.getByRole('button', { name: 'Рассказать наставнику' }));
    expect(useChatStore.getState().composer?.text).toBe('Передай наставнику: ');
    // подтверждение: наставник уже видит отметку, подробности — по желанию стажёра
    expect(screen.getByText(TOLD_TEXT)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Что мне делать дальше?' }));
    expect(useChatStore.getState().queued?.text).toBe('Что мне делать дальше?');
  });

  it('сегодняшняя отметка с сервера видна и меняется; «Хорошо» — короткая поддержка без кнопок помощи', async () => {
    mockFetch([
      { match: '/api/v1/pulse/me', body: { today, entries: [{ date: '2026-11-14', mood: 1 }, { date: today, mood: 4 }] } },
      { method: 'POST', match: '/api/v1/pulse', body: { date: today, mood: 2 } },
    ]);
    renderWithQuery(<PulseStrip />);
    expect(await screen.findByText('Сегодня: Хорошо.')).toBeInTheDocument();
    expect(screen.getByText(/Хороший день, чтобы закрыть следующий шаг/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Хорошо' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.queryByRole('button', { name: 'Рассказать наставнику' })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Так себе' }));
    expect(await screen.findByRole('button', { name: 'Рассказать наставнику' })).toBeInTheDocument();
    expect(screen.getByText('Сегодня: Так себе.')).toBeInTheDocument();
  });

  it('сервер без «Пульса» — блок не показывается', async () => {
    const spy = mockFetch([]);
    renderWithQuery(<PulseStrip />);
    await waitFor(() => expect(spy).toHaveBeenCalled());
    expect(screen.queryByText('Как вы сегодня?')).not.toBeInTheDocument();
  });
});
