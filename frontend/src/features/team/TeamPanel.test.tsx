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
  it('наставник видит, кто из стажёров просит помощи, и отмечает сообщение разобранным', async () => {
    const spy = mockFetch([
      ...base,
      { match: '/api/v1/team/actions?status=pending', body: [] },
      {
        match: '/api/v1/team/signals',
        body: [{ id: 'SIG-000001', user_id: 'intern-1', text: 'не работает VPN', topic: 'tech', done: false }],
      },
      { method: 'POST', match: '/api/v1/team/signals/SIG-000001/resolve', body: { ok: true } },
    ]);
    renderWithQuery(<TeamPanel role="buddy" />);
    expect(await screen.findByText('не работает VPN')).toBeInTheDocument();
    expect(screen.getByText('Стажёры просят помощи')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Отметить SIG-000001 разобранным' }));
    await waitFor(() =>
      expect(spy.mock.calls.some(([u]) => String(u).endsWith('/api/v1/team/signals/SIG-000001/resolve'))).toBe(true),
    );
  });

  it('руководитель видит заявки, прогресс стажёров и пробелы базы; согласует с комментарием', async () => {
    const spy = mockFetch([
      ...base,
      { match: '/api/v1/team/actions?status=pending', body: [absence, access] },
      { method: 'POST', match: '/api/v1/actions/ABS-000001/approve', body: { ...absence, status: 'approved' } },
    ]);
    renderWithQuery(<TeamPanel role="lead" />);
    expect(screen.getByText(/Загружаю данные команды/)).toBeInTheDocument();

    expect(await screen.findByText('Отгул 15.01.2030 – 16.01.2030')).toBeInTheDocument();
    expect(screen.getByText('Доступ к GitLab')).toBeInTheDocument();
    // автор заявки показан по имени, а не по id
    expect(screen.getAllByText('Стажёр Backend').length).toBeGreaterThan(0);
    expect(screen.getByRole('progressbar', { name: 'Прогресс: Стажёр Backend' })).toHaveAttribute('aria-valuenow', '25');
    expect(screen.getByText(/ждёт заявок: 1/)).toBeInTheDocument();
    expect(screen.getByText('Все шаги выполнены')).toBeInTheDocument();
    expect(await screen.findByText('Сколько платят в Авито?')).toBeInTheDocument();
    expect(screen.getByText('×3')).toBeInTheDocument();

    // у каждой заявки своё поле комментария — видно, к какой заявке он уйдёт
    await userEvent.type(screen.getByLabelText('Комментарий к заявке ABS-000001'), 'Хорошего отдыха');
    await userEvent.type(screen.getByLabelText('Комментарий к заявке REQ-000001'), 'Про доступ');
    await userEvent.click(screen.getByRole('button', { name: 'Согласовать ABS-000001' }));
    await waitFor(() => {
      const call = spy.mock.calls.find(([url]) => String(url).endsWith('/api/v1/actions/ABS-000001/approve'));
      expect(call).toBeDefined();
      expect(JSON.parse(call![1]?.body as string)).toEqual({ comment: 'Хорошего отдыха' });
    });
    // после решения очищается комментарий только этой заявки
    await waitFor(() => expect(screen.getByLabelText('Комментарий к заявке ABS-000001')).toHaveValue(''));
    expect(screen.getByLabelText('Комментарий к заявке REQ-000001')).toHaveValue('Про доступ');
    // заявка исчезает из списка — руководитель видит словами, что решение ушло
    expect(screen.getByRole('status')).toHaveTextContent(
      'Отгул 15.01.2030 – 16.01.2030 (ABS-000001) согласована. Стажёр увидит решение и ваш комментарий',
    );
  });

  it('комментарий уходит только с той заявкой, у которой он написан', async () => {
    const spy = mockFetch([
      ...base,
      { match: '/api/v1/team/actions?status=pending', body: [absence, access] },
      { method: 'POST', match: '/api/v1/actions/REQ-000001/reject', body: { ...access, status: 'rejected' } },
    ]);
    renderWithQuery(<TeamPanel role="lead" />);
    await userEvent.type(await screen.findByLabelText('Комментарий к заявке ABS-000001'), 'Это про отгул');
    await userEvent.click(screen.getByRole('button', { name: 'Отклонить REQ-000001' }));
    await waitFor(() => {
      const call = spy.mock.calls.find(([url]) => String(url).endsWith('/api/v1/actions/REQ-000001/reject'));
      expect(call).toBeDefined();
      expect(JSON.parse(call![1]?.body as string)).toEqual({ comment: '' });
    });
    expect(screen.getByLabelText('Комментарий к заявке ABS-000001')).toHaveValue('Это про отгул');
  });

  it('пока пробелы базы грузятся — «Загружаю», а не «ответил на все вопросы»', async () => {
    const answer = mockFetch([
      { match: '/api/v1/team/progress', body: [] },
      { match: '/api/v1/team/actions?status=pending', body: [] },
    ]);
    // /team/knowledge-gaps не отвечает — запрос «висит»
    vi.stubGlobal('fetch', (input: RequestInfo | URL, init?: RequestInit) =>
      String(input).endsWith('/team/knowledge-gaps') ? new Promise<Response>(() => {}) : answer(input, init),
    );
    renderWithQuery(<TeamPanel role="buddy" />);
    expect(await screen.findByText('Загружаю вопросы без ответа…')).toBeInTheDocument();
    expect(screen.queryByText(/ответил на все вопросы/)).not.toBeInTheDocument();
  });

  it('ошибка загрузки пробелов базы — ошибка с «Повторить», а не «ответил на все вопросы»', async () => {
    const spy = mockFetch([
      { match: '/api/v1/team/progress', body: [] },
      { match: '/api/v1/team/actions?status=pending', body: [] },
      { match: '/api/v1/team/knowledge-gaps', status: 500, body: { error: { code: 'internal', message: 'boom' } } },
    ]);
    renderWithQuery(<TeamPanel role="buddy" />);
    expect(await screen.findByText(/Не удалось загрузить вопросы без ответа/)).toBeInTheDocument();
    expect(screen.queryByText(/ответил на все вопросы/)).not.toBeInTheDocument();
    // остальной кабинет работает
    expect(screen.getByText('Новых заявок нет.')).toBeInTheDocument();
    const gapCalls = () => spy.mock.calls.filter(([u]) => String(u).endsWith('/team/knowledge-gaps')).length;
    const before = gapCalls();
    await userEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    await waitFor(() => expect(gapCalls()).toBeGreaterThan(before));
  });

  it('«метрики пользы»: плитки сводки по команде из /team/summary', async () => {
    mockFetch([
      ...base,
      { match: '/api/v1/team/actions?status=pending', body: [absence] },
      {
        match: '/api/v1/team/summary',
        body: {
          interns: 2,
          avg_percent_done: 62.5,
          steps_waiting: 1,
          actions: { pending: 3, approved: 1 },
          unanswered_topics: 4,
          unanswered_requests: 7,
        },
      },
    ]);
    renderWithQuery(<TeamPanel role="lead" />);
    const tiles = await screen.findByLabelText('Метрики команды');
    expect(tiles).toHaveTextContent('2Стажёров');
    expect(tiles).toHaveTextContent('63%Средний прогресс');
    expect(tiles).toHaveTextContent('3Заявок ждут решения');
    expect(tiles).toHaveTextContent('4Вопросов без ответа');
  });

  it('«Сэкономлено наставнику»: крупная плитка с минутами и расшифровкой', async () => {
    mockFetch([
      ...base,
      { match: '/api/v1/team/actions?status=pending', body: [] },
      {
        match: '/api/v1/team/summary',
        body: {
          interns: 2,
          avg_percent_done: 50,
          actions: {},
          unanswered_topics: 0,
          questions_closed: 37,
          signals_total: 4,
          refusals_total: 3,
          mentor_minutes_saved: 185,
          minutes_per_question: 5,
          closed_share: 0.84,
        },
      },
    ]);
    renderWithQuery(<TeamPanel role="lead" />);
    const hero = await screen.findByLabelText('Сэкономлено наставнику');
    expect(hero).toHaveTextContent('Бадди сэкономил наставнику ≈ 185 мин');
    expect(hero).toHaveTextContent('закрыто вопросов: 37 · передано наставнику: 4 · оценка: 5 мин на вопрос');
  });

  it('«Пульс команды»: сначала стажёры, которым нужно внимание, с причиной; точки за 7 дней', async () => {
    const days = (moods: (number | null)[]) => moods.map((mood, i) => ({ date: `2026-11-${String(9 + i).padStart(2, '0')}`, mood }));
    mockFetch([
      ...base,
      { match: '/api/v1/team/actions?status=pending', body: [] },
      {
        match: '/api/v1/team/pulse',
        body: [
          {
            user: { id: 'intern-3', name: 'Второй стажёр', role: 'intern', team_id: 'backend' },
            today: 5,
            avg7: 4.5,
            trend: 'up',
            risk: false,
            days: days([null, null, null, null, null, 4, 5]),
          },
          {
            user: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' },
            today: 1,
            avg7: 1.7,
            trend: 'down',
            risk: true,
            risk_reason: 'сегодня отметил(а) «Тяжело»',
            days: days([null, null, null, null, 2, 2, 1]),
            note: 'не понимаю задачу',
            note_date: '2026-11-15',
          },
        ],
      },
    ]);
    renderWithQuery(<TeamPanel role="buddy" />);
    expect(await screen.findByText('Пульс команды')).toBeInTheDocument();
    expect(screen.getByText(/видны только наставнику и руководителю команды/)).toBeInTheDocument();
    const rows = document.querySelectorAll('.pulse-row');
    expect(rows[0]).toHaveTextContent('Стажёр Backend');
    expect(rows[0]).toHaveTextContent('нужно внимание');
    expect(rows[0]).toHaveTextContent('Почему: сегодня отметил(а) «Тяжело»');
    expect(rows[0]).toHaveTextContent('сегодня: Тяжело');
    expect(rows[0]).toHaveTextContent('«не понимаю задачу»');
    expect(rows[0]).toHaveTextContent('среднее за 7 дней: 1,7 из 5 · настроение снижается');
    expect(rows[1]).toHaveTextContent('Второй стажёр');
    expect(rows[1]).not.toHaveTextContent('нужно внимание');
    expect(screen.getAllByRole('img', { name: /^Последние 7 дней:/ })[0]).toHaveAccessibleName(
      'Последние 7 дней: нет отметки, нет отметки, нет отметки, нет отметки, Так себе, Так себе, Тяжело',
    );
  });

  it('сводка без заявок и с null вместо actions — ноль, а не ошибка', async () => {
    mockFetch([
      ...base,
      { match: '/api/v1/team/actions?status=pending', body: [] },
      { match: '/api/v1/team/summary', body: { interns: 0, avg_percent_done: 0, actions: null, unanswered_topics: 0 } },
    ]);
    renderWithQuery(<TeamPanel role="buddy" />);
    expect(await screen.findByLabelText('Метрики команды')).toHaveTextContent('0Заявок ждут решения');
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
    expect(screen.queryByLabelText(/Комментарий к заявке/)).not.toBeInTheDocument();
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
    const spyFirst = mockFetch([
      ...base,
      { match: '/api/v1/team/actions?status=pending', body: [absence] },
      { method: 'POST', match: '/api/v1/actions/ABS-000001/approve', status: 409, body: { error: { code: 'conflict', message: 'action is not pending' } } },
    ]);
    const first = renderWithQuery(<TeamPanel role="lead" />);
    const pendingCalls = () => spyFirst.mock.calls.filter(([u]) => String(u).endsWith('/team/actions?status=pending')).length;
    const approveBtn = await screen.findByRole('button', { name: 'Согласовать ABS-000001' });
    const pendingBefore = pendingCalls();
    await userEvent.click(approveBtn);
    // вместо технического 'action is not pending' — понятный текст, и список перечитывается
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Заявка уже обработана, список обновлён');
    expect(alert).not.toHaveTextContent('action is not pending');
    await waitFor(() => expect(pendingCalls()).toBeGreaterThan(pendingBefore));
    first.unmount();

    const spy = mockFetch([]); // всё отвечает 404 — например, стажёр без прав
    renderWithQuery(<TeamPanel role="lead" />);
    expect(await screen.findByText(/Не удалось загрузить данные команды/)).toBeInTheDocument();
    const before = spy.mock.calls.length;
    await userEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    await waitFor(() => expect(spy.mock.calls.length).toBeGreaterThan(before));
  });
});
