import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { mockFetch, renderWithQuery } from '../../test/utils';
import { TaskDetails } from './TaskDetails';
import { TasksView } from './TasksView';

afterEach(() => vi.unstubAllGlobals());

const task = (id: string, status: string, extra: object = {}) => ({
  id,
  team: 'backend',
  title: `Задача ${id}`,
  summary: 'Описание',
  steps: ['Шаг один', 'Шаг два'],
  definition_of_done: 'Тесты зелёные',
  difficulty: 2,
  tags: ['go', 'testing'],
  popularity: 20,
  estimate_hours: 4,
  knowledge: 'code-review.md',
  status,
  ...extra,
});
const recs = {
  source: 'model',
  model: 'logreg-v1',
  items: [{ ...task('be-02', 'todo'), score: 0.74, reasons: ['Знакомые технологии: go, testing'] }],
};

describe('TasksView', () => {
  it('рекомендации с объяснением и каталог с фильтром по статусу', async () => {
    mockFetch([
      { match: '/api/v1/tasks', body: [task('be-01', 'done'), task('be-02', 'todo'), task('be-03', 'in_progress')] },
      { match: '/api/v1/recommendations?limit=3', body: recs },
    ]);
    renderWithQuery(<TasksView />);
    expect(await screen.findByText('Знакомые технологии: go, testing')).toBeInTheDocument();
    expect(screen.getByText('74%')).toBeInTheDocument();
    expect(screen.getByText(/Подобрано моделью рекомендаций/)).toBeInTheDocument();
    expect(await screen.findAllByRole('link', { name: /Задача be-0/ })).toHaveLength(4);

    await userEvent.click(screen.getByRole('button', { name: 'В работе · 1' }));
    expect(screen.getAllByRole('link', { name: /Задача be-0/ }).map((l) => l.textContent)).toEqual(['Задача be-02', 'Задача be-03']);
    await userEvent.click(screen.getByRole('button', { name: 'Отложены · 0' }));
    expect(screen.getByText('В этом статусе задач нет.')).toBeInTheDocument();
  });

  it('без модели честно говорит про правило; ошибка — «Повторить»', async () => {
    mockFetch([
      { match: '/api/v1/tasks', status: 500, body: {} },
      { match: '/api/v1/recommendations?limit=3', body: { ...recs, source: 'rule_based', items: recs.items } },
    ]);
    renderWithQuery(<TasksView />);
    expect(await screen.findByText(/простым правилом/)).toBeInTheDocument();
    expect(await screen.findByText(/Не удалось загрузить задачи/)).toBeInTheDocument();
  });

  it('пустые рекомендации и сбой рекомендаций', async () => {
    mockFetch([
      { match: '/api/v1/tasks', body: [] },
      { match: '/api/v1/recommendations?limit=3', body: { source: 'model', items: [] } },
    ]);
    const { unmount } = renderWithQuery(<TasksView />);
    expect(await screen.findByText(/Все задачи каталога уже взяты/)).toBeInTheDocument();
    unmount();
    mockFetch([
      { match: '/api/v1/tasks', body: [] },
      { match: '/api/v1/recommendations?limit=3', status: 403, body: { error: { code: 'forbidden', message: 'x' } } },
    ]);
    renderWithQuery(<TasksView />);
    expect(await screen.findByText(/Не удалось получить рекомендации/)).toBeInTheDocument();
  });
});

describe('TaskDetails', () => {
  it('показывает шаги и критерий готовности; «Взять в работу» → «Отметить сделанной»', async () => {
    const fetch = mockFetch([
      { match: '/api/v1/tasks/be-02', body: task('be-02', 'todo') },
      { method: 'POST', match: '/api/v1/tasks/be-02/start', body: task('be-02', 'in_progress', { started_at: '2026-11-16T10:00:00Z' }) },
    ]);
    const onAsk = vi.fn();
    renderWithQuery(<TaskDetails taskId="be-02" onAsk={onAsk} />);
    expect(await screen.findByRole('heading', { name: 'Задача be-02' })).toBeInTheDocument();
    expect(screen.getByText('Шаг два')).toBeInTheDocument();
    expect(screen.getByText('Тесты зелёные')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Прочитать в базе знаний' })).toHaveAttribute('href', '/knowledge/code-review.md');

    await userEvent.click(screen.getByRole('button', { name: 'Спросить Бадди' }));
    expect(onAsk).toHaveBeenCalledWith('Как подступиться к задаче «Задача be-02»?');

    await userEvent.click(screen.getByRole('button', { name: 'Взять в работу' }));
    expect(await screen.findByRole('button', { name: 'Отметить сделанной' })).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('Задача в работе');
    expect(screen.getByText(/Взята 16.11.2026/)).toBeInTheDocument();
    expect(fetch.mock.calls.some(([u, i]) => String(u).endsWith('/start') && i?.method === 'POST')).toBe(true);
  });

  it('409 — понятное сообщение сервера', async () => {
    mockFetch([
      { match: '/api/v1/tasks/be-03', body: task('be-03', 'in_progress') },
      {
        method: 'POST',
        match: '/api/v1/tasks/be-03/complete',
        status: 409,
        body: { error: { code: 'conflict', message: 'нельзя изменить задачу в статусе «сделано»' } },
      },
    ]);
    renderWithQuery(<TaskDetails taskId="be-03" onAsk={() => {}} />);
    await userEvent.click(await screen.findByRole('button', { name: 'Отметить сделанной' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('нельзя изменить задачу');
  });

  it('чужая или несуществующая задача — 404 со ссылкой на список; сбой — «Повторить»', async () => {
    mockFetch([{ match: '/api/v1/tasks/fe-01', status: 404, body: { error: { code: 'not_found', message: 'task not found' } } }]);
    const { unmount } = renderWithQuery(<TaskDetails taskId="fe-01" onAsk={() => {}} />);
    expect(await screen.findByText('Такой задачи нет в каталоге вашей команды.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'К списку задач' })).toHaveAttribute('href', '/tasks');
    unmount();
    mockFetch([{ match: '/api/v1/tasks/be-01', status: 500, body: {} }]);
    renderWithQuery(<TaskDetails taskId="be-01" onAsk={() => {}} />);
    expect(await screen.findByRole('button', { name: 'Повторить' })).toBeInTheDocument();
  });

  it('сделанная задача — без кнопок смены статуса, отложенную можно вернуть', async () => {
    mockFetch([{ match: '/api/v1/tasks/be-01', body: task('be-01', 'done', { done_at: '2026-11-16T12:00:00Z' }) }]);
    const { unmount } = renderWithQuery(<TaskDetails taskId="be-01" onAsk={() => {}} />);
    expect(await screen.findByText(/сделана 16.11.2026/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Взять|Отложить|Отметить/ })).not.toBeInTheDocument();
    unmount();
    mockFetch([
      { match: '/api/v1/tasks/be-04', body: task('be-04', 'skipped') },
      { method: 'POST', match: '/api/v1/tasks/be-04/reopen', body: task('be-04', 'todo') },
    ]);
    renderWithQuery(<TaskDetails taskId="be-04" onAsk={() => {}} />);
    await userEvent.click(await screen.findByRole('button', { name: 'Вернуть в список' }));
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Задача снова в списке.'));
  });
});
