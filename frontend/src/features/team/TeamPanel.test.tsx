import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TeamPanel } from './TeamPanel';
import { mockFetch, renderWithQuery } from '../../test/utils';

afterEach(() => vi.unstubAllGlobals());

const intern = (id: string, name: string, pct: number, waiting = false) => ({
  user: { id, name, role: 'intern', team_id: 'backend' },
  items: [
    { id: `${id}-1`, title: 'Получить доступы', stage: 'week1', done: false, status: waiting ? 'waiting' : 'todo' },
  ],
  next_step: pct === 100 ? '' : 'Прочитать Playbook',
  percent_done: pct,
});

const absence = { id: 'ABS-000001', type: 'absence', user_id: 'intern-1', date_from: '2030-01-15', date_to: '2030-01-16', status: 'pending' };
const access = { id: 'REQ-000001', type: 'access', resource: 'gitlab', user_id: 'intern-3', date_from: '', date_to: '', status: 'pending' };

const base = [
  { match: '/api/v1/team/progress', body: [intern('intern-1', 'Стажёр Backend', 25, true), intern('intern-3', 'Второй стажёр', 100)] },
  { match: '/api/v1/team/knowledge-gaps', body: [{ question: 'Сколько платят в Авито?', count: 3 }] },
];

describe('TeamPanel', () => {
  it('руководитель видит заявки, прогресс стажёров и пробелы базы; согласует с комментарием', async () => {
    const spy = mockFetch([
      ...base,
      { match: '/api/v1/team/actions?status=pending', body: [absence, access] },
      { method: 'POST', match: '/api/v1/actions/ABS-000001/approve', body: { ...absence, status: 'approved' } },
    ]);
    renderWithQuery(<TeamPanel role="lead" />);
    expect(screen.getByText(/Загружаю данные команды/)).toBeInTheDocument();

    expect(await screen.findByText('Отгул 15.01.2030 – 16.01.2030')).toBeInTheDocument();
    expect(screen.getByText('Доступ к gitlab')).toBeInTheDocument();
    // автор заявки показан по имени, а не по id
    expect(screen.getAllByText('Стажёр Backend').length).toBeGreaterThan(0);
    expect(screen.getByRole('progressbar', { name: 'Прогресс: Стажёр Backend' })).toHaveAttribute('aria-valuenow', '25');
    expect(screen.getByText(/ждёт заявок: 1/)).toBeInTheDocument();
    expect(screen.getByText('Все шаги выполнены')).toBeInTheDocument();
    expect(await screen.findByText('Сколько платят в Авито?')).toBeInTheDocument();
    expect(screen.getByText('×3')).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText('Комментарий к решению'), 'Хорошего отдыха');
    await userEvent.click(screen.getByRole('button', { name: 'Согласовать ABS-000001' }));
    await waitFor(() => {
      const call = spy.mock.calls.find(([url]) => String(url).endsWith('/api/v1/actions/ABS-000001/approve'));
      expect(call).toBeDefined();
      expect(JSON.parse(call![1]?.body as string)).toEqual({ comment: 'Хорошего отдыха' });
    });
    // после решения комментарий очищается
    await waitFor(() => expect(screen.getByLabelText('Комментарий к решению')).toHaveValue(''));
  });

  it('руководитель отклоняет заявку', async () => {
    const spy = mockFetch([
      ...base,
      { match: '/api/v1/team/actions?status=pending', body: [access] },
      { method: 'POST', match: '/api/v1/actions/REQ-000001/reject', body: { ...access, status: 'rejected' } },
    ]);
    renderWithQuery(<TeamPanel role="lead" />);
    await userEvent.click(await screen.findByRole('button', { name: 'Отклонить REQ-000001' }));
    await waitFor(() =>
      expect(spy.mock.calls.some(([url]) => String(url).endsWith('/api/v1/actions/REQ-000001/reject'))).toBe(true),
    );
  });

  it('наставник видит заявки без кнопок решения', async () => {
    mockFetch([...base, { match: '/api/v1/team/actions?status=pending', body: [absence] }]);
    renderWithQuery(<TeamPanel role="buddy" />);
    expect(await screen.findByText('Отгул 15.01.2030 – 16.01.2030')).toBeInTheDocument();
    expect(screen.getByText(/наставник: прогресс стажёров/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Согласовать/ })).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Комментарий к решению')).not.toBeInTheDocument();
  });

  it('пустые списки: нет заявок, стажёров и пробелов', async () => {
    mockFetch([
      { match: '/api/v1/team/progress', body: [] },
      { match: '/api/v1/team/knowledge-gaps', body: null },
      { match: '/api/v1/team/actions?status=pending', body: null },
    ]);
    renderWithQuery(<TeamPanel role="lead" />);
    expect(await screen.findByText('Новых заявок нет.')).toBeInTheDocument();
    expect(screen.getByText('В команде пока нет стажёров.')).toBeInTheDocument();
    expect(await screen.findByText(/ответил на все вопросы/)).toBeInTheDocument();
  });

  it('ошибка согласования показывается, ошибка загрузки — с кнопкой «Повторить»', async () => {
    mockFetch([
      ...base,
      { match: '/api/v1/team/actions?status=pending', body: [absence] },
      { method: 'POST', match: '/api/v1/actions/ABS-000001/approve', status: 409, body: { error: { code: 'conflict', message: 'action is not pending' } } },
    ]);
    const first = renderWithQuery(<TeamPanel role="lead" />);
    await userEvent.click(await screen.findByRole('button', { name: 'Согласовать ABS-000001' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('action is not pending');
    first.unmount();

    const spy = mockFetch([]); // всё отвечает 404 — например, стажёр без прав
    renderWithQuery(<TeamPanel role="lead" />);
    expect(await screen.findByText(/Не удалось загрузить данные команды/)).toBeInTheDocument();
    const before = spy.mock.calls.length;
    await userEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    await waitFor(() => expect(spy.mock.calls.length).toBeGreaterThan(before));
  });
});
