import { screen } from '@testing-library/react';
import { Desk } from './Desk';
import { useChatStore } from '../../store/chat';
import { mockFetch, renderWithQuery } from '../../test/utils';

beforeEach(() => useChatStore.getState().setUser('intern.backend'));
afterEach(() => vi.unstubAllGlobals());

const noop = () => {};
const progress = { items: [{ id: 's1', title: 'Прочитать Playbook', stage: 'week1', done: false }], next_step: 'Прочитать Playbook', percent_done: 50 };

describe('Desk', () => {
  it('карточка заявок показывает самую новую заявку (API отдаёт новые сверху)', async () => {
    mockFetch([
      { match: '/api/v1/onboarding/progress', body: progress },
      {
        match: '/api/v1/actions',
        body: [
          { id: 'REQ-000001', type: 'access', resource: 'gitlab', date_from: '', date_to: '', status: 'approved' },
          { id: 'ABS-000001', type: 'absence', date_from: '2030-01-15', date_to: '2030-01-15', status: 'pending' },
        ],
      },
    ]);
    renderWithQuery(<Desk onOpenPlan={noop} onOpenActions={noop} onReport={noop} />);
    expect(await screen.findByText('Последняя: Доступ к GitLab — согласована')).toBeInTheDocument();
    expect(screen.getByText('1 заявка ждёт согласования')).toBeInTheDocument();
    expect(screen.getByText('пройдено 50%')).toBeInTheDocument();
  });

  it('на рабочем столе — «Как вы сегодня?» и «Достижения»', async () => {
    mockFetch([
      { match: '/api/v1/onboarding/progress', body: progress },
      { match: '/api/v1/actions', body: [] },
      { match: '/api/v1/pulse/me', body: { today: '2026-11-15', entries: [] } },
      {
        match: '/api/v1/achievements',
        body: [{ id: 'first_question', title: 'Первый вопрос', description: 'd', unlocked: false, hint: 'Спросите Бадди' }],
      },
    ]);
    renderWithQuery(<Desk onOpenPlan={noop} onOpenActions={noop} onReport={noop} />);
    expect(await screen.findByRole('group', { name: 'Как вы сегодня?' })).toBeInTheDocument();
    expect(await screen.findByText('Следующее: Первый вопрос.')).toBeInTheDocument();
  });

  it('пока заявки грузятся — не пишет «Заявок пока нет»; при ошибке — честно говорит об этом', async () => {
    mockFetch([{ match: '/api/v1/onboarding/progress', body: progress }]); // /actions → 404
    renderWithQuery(<Desk onOpenPlan={noop} onOpenActions={noop} onReport={noop} />);
    expect(screen.getByText('Загружаю заявки…')).toBeInTheDocument();
    expect(screen.queryByText('Заявок пока нет')).not.toBeInTheDocument();
    expect(await screen.findByText('Заявки пока недоступны')).toBeInTheDocument();
  });
});
