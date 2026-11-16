import { screen } from '@testing-library/react';
import { mockFetch, renderWithQuery } from '../../test/utils';
import { InternCard } from './InternCard';

afterEach(() => vi.unstubAllGlobals());

const card = {
  user: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' },
  progress: { items: [{ id: 'c1', title: 'Получить доступы', stage: 'week1', done: false, status: 'waiting' }], next_step: '', percent_done: 50 },
  pulse: { today: 2, avg7: 2.5, trend: 'down', risk: true, risk_reason: 'три дня подряд «тяжело»', days: [] },
  actions: [{ id: 'REQ-000001', type: 'access', resource: 'gitlab', status: 'pending' }],
  signals: [{ id: 'sig-1', user_id: 'intern-1', text: 'не запускается проект', topic: 'blocker', done: false, at: '2026-11-16T09:00:00Z' }],
  tasks: {
    summary: { total: 12, in_progress: 1, done: 2, skipped: 0 },
    active: [{ id: 'be-01', title: 'Запустить сервис', status: 'in_progress', difficulty: 1, tags: ['go'] }],
    recommendations: { source: 'model', items: [{ id: 'be-02', title: 'Покрыть тестами', score: 0.7, reasons: ['По силам'], difficulty: 2, tags: [] }] },
  },
};

describe('InternCard', () => {
  it('сводка, риск «Пульса», обращения, заявки и задачи стажёра', async () => {
    mockFetch([{ match: '/api/v1/team/interns/intern-1', body: card }]);
    renderWithQuery(<InternCard internId="intern-1" canDecide />);
    expect(await screen.findByRole('heading', { name: 'Стажёр Backend' })).toBeInTheDocument();
    expect(screen.getByText('50%')).toBeInTheDocument();
    expect(screen.getByText(/Нужно внимание: три дня подряд/)).toBeInTheDocument();
    expect(screen.getByText('не запускается проект')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Согласовать в кабинете команды (1)' })).toHaveAttribute('href', '/team');
    expect(screen.getByText('Запустить сервис')).toBeInTheDocument();
    // наставнику раздел задач стажёра недоступен — рекомендация без ссылки
    expect(screen.getByText('Покрыть тестами').tagName).toBe('SPAN');
  });

  it('наставник не согласует; стажёр другой команды — 404', async () => {
    mockFetch([{ match: '/api/v1/team/interns/intern-1', body: { ...card, tasks: undefined, signals: [], actions: card.actions } }]);
    const { unmount } = renderWithQuery(<InternCard internId="intern-1" canDecide={false} />);
    expect(await screen.findByText('Ждут решения руководителя: 1')).toBeInTheDocument();
    expect(screen.getByText('Стажёр пока не обращался.')).toBeInTheDocument();
    unmount();
    mockFetch([{ match: '/api/v1/team/interns/intern-2', status: 404, body: { error: { code: 'not_found', message: 'x' } } }]);
    const second = renderWithQuery(<InternCard internId="intern-2" canDecide />);
    expect(await screen.findByText('Такого стажёра нет в вашей команде.')).toBeInTheDocument();
    second.unmount();
    mockFetch([{ match: '/api/v1/team/interns/intern-3', status: 500, body: {} }]);
    renderWithQuery(<InternCard internId="intern-3" canDecide />);
    expect(await screen.findByText('Не удалось загрузить карточку.')).toBeInTheDocument();
  });
});
