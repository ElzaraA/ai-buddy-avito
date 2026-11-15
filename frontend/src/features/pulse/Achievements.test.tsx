import { screen, waitFor, within } from '@testing-library/react';
import { Achievements } from './Achievements';
import { useChatStore } from '../../store/chat';
import { mockFetch, renderWithQuery } from '../../test/utils';

beforeEach(() => useChatStore.getState().setUser('intern.backend'));
afterEach(() => vi.unstubAllGlobals());

const a = (id: string, title: string, unlocked: boolean, hint = `подсказка ${id}`) => ({
  id,
  title,
  description: `описание ${id}`,
  unlocked,
  hint,
  ...(unlocked ? { unlocked_at: '2026-11-12T10:00:00Z' } : {}),
});

describe('Achievements', () => {
  it('полученные и закрытые различаются текстом; следующее достижимое выделено', async () => {
    mockFetch([
      {
        match: '/api/v1/achievements',
        body: [
          a('first_question', 'Первый вопрос', true),
          a('first_action', 'Первая заявка', false, 'Попросите Бадди оформить доступ'),
          a('brave_ask', 'Смелость спросить', false),
        ],
      },
    ]);
    renderWithQuery(<Achievements />);
    expect(await screen.findByRole('heading', { name: 'Достижения' })).toBeInTheDocument();
    expect(screen.getByText('получено 1 из 3')).toBeInTheDocument();
    expect(screen.getByText('Следующее: Первая заявка.')).toBeInTheDocument();

    const items = screen.getAllByRole('listitem');
    expect(items[0]).toHaveClass('badge-card--on');
    expect(within(items[0]).getByText(/^получено/)).toBeInTheDocument();
    expect(items[1]).toHaveClass('badge-card--next');
    expect(within(items[1]).getByText('следующее')).toBeInTheDocument();
    expect(within(items[1]).getByText('Попросите Бадди оформить доступ')).toBeInTheDocument();
    expect(within(items[2]).getByText('пока закрыто')).toBeInTheDocument();
    expect(within(items[2]).getByText('подсказка brave_ask')).toBeInTheDocument();
  });

  it('всё получено — поздравление вместо «Следующее»', async () => {
    mockFetch([{ match: '/api/v1/achievements', body: [a('first_question', 'Первый вопрос', true)] }]);
    renderWithQuery(<Achievements />);
    expect(await screen.findByText('Все достижения получены!')).toBeInTheDocument();
    expect(screen.queryByText(/Следующее:/)).not.toBeInTheDocument();
  });

  it('сервер без достижений — блок не показывается', async () => {
    const spy = mockFetch([]);
    renderWithQuery(<Achievements />);
    await waitFor(() => expect(spy).toHaveBeenCalled());
    expect(screen.queryByText('Достижения')).not.toBeInTheDocument();
  });
});
