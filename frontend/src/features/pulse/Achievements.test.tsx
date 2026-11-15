import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Achievements, recentUnlocked } from './Achievements';
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
    // свёрнуто: полученное видно значком в строке, полный список — по кнопке
    expect(within(screen.getByRole('list', { name: 'Последние полученные' })).getByText('Первый вопрос')).toBeInTheDocument();
    expect(screen.queryByRole('list', { name: 'Все достижения' })).not.toBeInTheDocument();
    const toggle = screen.getByRole('button', { name: 'Все достижения' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    await userEvent.click(toggle);
    expect(screen.getByRole('button', { name: 'Свернуть достижения' })).toHaveAttribute('aria-expanded', 'true');

    const items = within(screen.getByRole('list', { name: 'Все достижения' })).getAllByRole('listitem');
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

  it('в свёрнутой строке — самые свежие полученные, не больше трёх', () => {
    const list = [
      { ...a('q', 'Первый вопрос', true), unlocked_at: '2026-11-10T10:00:00Z' },
      { ...a('f', 'Первая заявка', true), unlocked_at: '2026-11-11T10:00:00Z' },
      a('x', 'Закрытое', false),
      { ...a('acc', 'Доступы получены', true), unlocked_at: '2026-11-12T10:00:00Z' },
      { ...a('half', 'Половина пути', true), unlocked_at: '2026-11-12T10:00:00Z' },
    ];
    expect(recentUnlocked(list).map((x) => x.id)).toEqual(['acc', 'half', 'f']);
  });

  it('сервер без достижений — блок не показывается', async () => {
    const spy = mockFetch([]);
    renderWithQuery(<Achievements />);
    await waitFor(() => expect(spy).toHaveBeenCalled());
    expect(screen.queryByText('Достижения')).not.toBeInTheDocument();
  });
});
