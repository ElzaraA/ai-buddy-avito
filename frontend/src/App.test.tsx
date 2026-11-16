import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { App } from './App';
import { getDemoUser } from './api/client';
import { useChatStore } from './store/chat';
import { FakeEventSource, mockFetch, renderWithQuery } from './test/utils';

beforeEach(() => {
  window.history.replaceState(null, '', '/');
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
    expect(await screen.findByText('Стажёр Backend', { selector: '.me__name' })).toBeInTheDocument();
    expect(screen.getByText('команда backend')).toBeInTheDocument();
    expect(screen.getByLabelText('Чат с AI-buddy')).toBeInTheDocument();
    expect(await screen.findByText('Моя адаптация')).toBeInTheDocument();
    expect(screen.getByText(/не являются регламентами Авито/)).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText('Войти как'), 'intern.other');
    await waitFor(() => expect(getDemoUser()).toBe('intern.other'));
    expect(useChatStore.getState().user).toBe('intern.other');
  });

  it('в переключателе есть руководитель команды frontend', async () => {
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/onboarding/progress', body: { items: [], next_step: '' } },
    ]);
    renderWithQuery(<App />);
    expect(screen.getByRole('option', { name: 'Руководитель Frontend' })).toHaveValue('lead.frontend');
    await userEvent.selectOptions(screen.getByLabelText('Войти как'), 'lead.frontend');
    await waitFor(() => expect(getDemoUser()).toBe('lead.frontend'));
  });

  it('оболочка: «Сообщить наставнику» в меню подставляет заготовку в чат, карточка открывает план', async () => {
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/onboarding/progress', body: { items: [], next_step: 'Прочитать Playbook' } },
    ]);
    renderWithQuery(<App />);
    expect(screen.getByRole('button', { name: 'Рабочий стол' })).toHaveAttribute('aria-current', 'page');

    const menu = screen.getByRole('navigation', { name: 'Разделы' });
    await userEvent.click(within(menu).getByRole('button', { name: 'Сообщить наставнику' }));
    expect(screen.getByLabelText('Сообщение')).toHaveValue('Передай наставнику: ');
    expect(screen.getByLabelText('Сообщение')).toHaveFocus();

    await userEvent.click(await screen.findByRole('button', { name: /Следующий шаг: Прочитать Playbook/ }));
    const nav = screen.getByRole('navigation', { name: 'Разделы' });
    expect(within(nav).getByRole('button', { name: 'Мой план' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByLabelText('Чат с AI-buddy').closest('.chat-card')).not.toBeVisible();
  });

  it('навигация одна — боковое меню; пункт меню и заголовок раздела называются одинаково', async () => {
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/onboarding/progress', body: { items: [], next_step: '' } },
      { match: '/api/v1/actions', body: [] },
    ]);
    renderWithQuery(<App />);
    expect(screen.queryByRole('group', { name: 'Вкладки' })).not.toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: 'Разделы' });
    for (const name of ['Рабочий стол', 'Мой план', 'Мои заявки', 'Что умеет Бадди']) {
      await userEvent.click(within(nav).getByRole('button', { name }));
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(name);
      expect(screen.getAllByRole('button', { name })).toHaveLength(1);
    }
  });

  it('пока /me не ответил, наставнику не показывается стол стажёра (роль — по логину)', async () => {
    useChatStore.getState().setUser('mentor.backend');
    const spy = mockFetch([{ match: '/api/v1/team/progress', body: [] }]); // /me → 404
    renderWithQuery(<App />);
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Кабинет команды');
    expect(screen.queryByRole('button', { name: /Открыть план/ })).not.toBeInTheDocument();
    await waitFor(() => expect(spy).toHaveBeenCalled());
    // запросы стажёра (на них сервер ответил бы наставнику 403) не уходят
    expect(spy.mock.calls.some(([u]) => /\/pulse\/me|\/achievements|\/onboarding\/progress/.test(String(u)))).toBe(false);
  });

  it('руководителю вместо плана адаптации показывается кабинет команды', async () => {
    useChatStore.getState().setUser('lead.backend');
    mockFetch([
      { match: '/api/v1/me', body: { id: 'lead-1', name: 'Руководитель Backend', role: 'lead', team_id: 'backend' } },
      { match: '/api/v1/team/progress', body: [] },
      { match: '/api/v1/team/actions?status=pending', body: [] },
      { match: '/api/v1/team/knowledge-gaps', body: [] },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByText('Моя команда')).toBeInTheDocument();
    expect(screen.getByText(/руководитель: согласование заявок/)).toBeInTheDocument();
    expect(screen.queryByText('Моя адаптация')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Чат с AI-buddy')).toBeInTheDocument();
  });
  it('расписание: напоминание приходит, пока открыт другой раздел, — счётчик на чате и сообщение в чате', async () => {
    mockFetch([
      {
        match: '/api/v1/me',
        body: {
          id: 'intern-1',
          name: 'Стажёр Backend',
          role: 'intern',
          team_id: 'backend',
        },
      },
      {
        match: '/api/v1/onboarding/progress',
        body: { items: [], next_step: '' },
      },
      {
        match: /\/api\/v1\/schedule\?days=7$/,
        body: { from: '', to: '', items: [] },
      },
      {
        match: '/api/v1/reminders',
        body: [
          {
            id: 'standup@x',
            title: 'Стендап команды',
            kind: 'standup',
            start: '2026-11-16T11:00:00+03:00',
            end: '2026-11-16T11:15:00+03:00',
            text: 'Через 15 минут — Стендап команды в 11:00.',
          },
        ],
      },
      {
        method: 'POST',
        match: '/api/v1/reminders/standup%40x/ack',
        body: null,
      },
    ]);
    renderWithQuery(<App />);
    const nav = screen.getByRole('navigation', { name: 'Разделы' });
    // чат открыт — напоминание сразу видно и счётчик не нужен
    expect(await screen.findByText('Через 15 минут — Стендап команды в 11:00.')).toBeVisible();
    expect(within(nav).queryByLabelText(/новых напоминаний/)).not.toBeInTheDocument();

    // в другом разделе новое напоминание отмечается счётчиком на пункте с чатом
    await userEvent.click(within(nav).getByRole('button', { name: 'Расписание' }));
    expect(await screen.findByText('На ближайшую неделю встреч нет.')).toBeInTheDocument();
    act(() => useChatStore.getState().addReminder('retro@x', 'Через 15 минут — Ретро спринта в 17:00.'));
    expect(await within(nav).findByLabelText('новых напоминаний: 1')).toBeInTheDocument();

    await userEvent.click(within(nav).getByRole('button', { name: /Рабочий стол/ }));
    expect(screen.getByText('Через 15 минут — Ретро спринта в 17:00.')).toBeVisible();
    await waitFor(() => expect(within(nav).queryByLabelText(/новых напоминаний/)).not.toBeInTheDocument());
  });

  it('прямой URL раздела переживает загрузку и Back возвращает в предыдущий раздел', async () => {
    window.history.replaceState(null, '', '/schedule');
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/schedule?days=7', body: { from: '', to: '', items: [] } },
      { match: '/api/v1/reminders', body: [] },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Расписание');
    await userEvent.click(screen.getByRole('button', { name: 'Мой план' }));
    expect(window.location.pathname).toBe('/plan');
    window.history.back();
    await waitFor(() => expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Расписание'));
  });

  it('прямой маршрут /chat доступен стажёру и показывает Buddy Chat', async () => {
    window.history.replaceState(null, '', '/chat');
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/reminders', body: [] },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Чат с Бадди');
    expect(screen.getByLabelText('Чат с AI-buddy')).toBeVisible();
  });

  it('профиль загружает текущего пользователя и его прогресс через API', async () => {
    window.history.replaceState(null, '', '/profile');
    const fetch = mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/onboarding/progress', body: { items: [{ id: 's1', title: 'Первый шаг', stage: 'week1', done: true }], next_step: '' } },
      { match: '/api/v1/pulse/me', body: { today: '2026-11-16', entries: [] } },
      { match: '/api/v1/reminders', body: [] },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByRole('heading', { name: 'Стажёр Backend' })).toBeInTheDocument();
    expect(await screen.findByText('100% выполнено')).toBeInTheDocument();
    expect(fetch.mock.calls.some(([url]) => String(url).endsWith('/api/v1/me'))).toBe(true);
  });

  it('сервер не узнал учётку (401) — экран входа с возвратом на открытый адрес', async () => {
    window.history.replaceState(null, '', '/profile');
    mockFetch([
      { match: '/api/v1/me', status: 401, body: { error: { code: 'unauthorized', message: 'unknown user' } } },
      { match: '/api/v1/reminders', body: [] },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByRole('heading', { name: 'Войти в демо' })).toBeInTheDocument();
    await waitFor(() => expect(window.location.pathname + window.location.search).toBe('/login?next=%2Fprofile'));
  });

  it('сбой сервера при загрузке профиля — ошибка и «Повторить»', async () => {
    window.history.replaceState(null, '', '/profile');
    mockFetch([
      { match: '/api/v1/me', status: 500, body: {} },
      { match: '/api/v1/reminders', body: [] },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByText('Не удалось загрузить текущего пользователя.', {}, { timeout: 4000 })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument();
  });

  it('показывает 403 для раздела команды стажёру и 404 для неизвестного URL', async () => {
    window.history.replaceState(null, '', '/team');
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/reminders', body: [] },
    ]);
    const first = renderWithQuery(<App />);
    expect(await screen.findByRole('heading', { name: 'Доступ запрещён' })).toBeInTheDocument();
    first.unmount();

    window.history.replaceState(null, '', '/missing-route');
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/reminders', body: [] },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByRole('heading', { name: 'Страница не найдена' })).toBeInTheDocument();
  });

  it('деталь шага использует API и не позволяет вручную закрыть шаг ожидания доступа', async () => {
    window.history.replaceState(null, '', '/plan/access-step');
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      {
        match: '/api/v1/onboarding/progress',
        body: {
          items: [{ id: 'access-step', title: 'Получить доступы', stage: 'week1', done: false, status: 'waiting', kind: 'access' }],
          next_step: 'Получить доступы',
        },
      },
      { match: '/api/v1/reminders', body: [] },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByRole('heading', { name: 'Получить доступы' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Отметить выполненным' })).not.toBeInTheDocument();
    expect(screen.getByText(/после решения по заявке/)).toBeInTheDocument();
  });

  it('без входа — экран входа, после выбора учётки открывается запрошенный адрес', async () => {
    useChatStore.setState({ signedIn: false });
    window.history.replaceState(null, '', '/knowledge');
    mockFetch([
      { match: '/api/v1/me', body: { id: 'mentor-1', name: 'Наставник Backend', role: 'buddy', team_id: 'backend' } },
      { match: '/api/v1/reminders', body: [] },
      { match: '/api/v1/knowledge', body: { documents: [{ id: 'mentors.md', title: 'Оценка стажёра', stages: ['week4'] }] } },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByRole('heading', { name: 'Войти в демо' })).toBeInTheDocument();
    expect(window.location.search).toBe('?next=%2Fknowledge');
    await userEvent.click(screen.getByRole('button', { name: /Наставник Backend/ }));
    expect(await screen.findByRole('link', { name: 'Оценка стажёра' })).toBeInTheDocument();
    expect(window.location.pathname).toBe('/knowledge');
    useChatStore.getState().setUser('intern.backend');
  });

  it('роли и детальные адреса: наставнику не открыть задачи стажёра, стажёру — карточку коллеги', async () => {
    useChatStore.getState().setUser('mentor.backend');
    window.history.replaceState(null, '', '/tasks/be-02');
    mockFetch([
      { match: '/api/v1/me', body: { id: 'mentor-1', name: 'Наставник Backend', role: 'buddy', team_id: 'backend' } },
      { match: '/api/v1/reminders', body: [] },
    ]);
    const first = renderWithQuery(<App />);
    expect(await screen.findByRole('heading', { name: 'Доступ запрещён' })).toBeInTheDocument();
    first.unmount();

    useChatStore.getState().setUser('intern.backend');
    window.history.replaceState(null, '', '/team/interns/intern-3');
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/reminders', body: [] },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByRole('heading', { name: 'Доступ запрещён' })).toBeInTheDocument();
  });

  it('переходы по ссылкам и «Назад»: задачи → задача → обратно, чат не теряется', async () => {
    window.history.replaceState(null, '', '/tasks');
    const task = { id: 'be-02', team: 'backend', title: 'Покрыть тестами', status: 'todo', difficulty: 2, tags: ['go'], steps: ['a'] };
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/reminders', body: [] },
      { match: '/api/v1/tasks', body: [task] },
      { match: '/api/v1/tasks/be-02', body: task },
      { match: /\/api\/v1\/recommendations/, body: { source: 'model', items: [{ ...task, score: 0.7, reasons: ['По силам'] }] } },
    ]);
    renderWithQuery(<App />);
    expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Задачи');
    const nav = screen.getByRole('navigation', { name: 'Разделы' });
    expect(within(nav).getByRole('button', { name: 'Задачи' })).toHaveAttribute('aria-current', 'page');
    await userEvent.click((await screen.findAllByRole('link', { name: 'Покрыть тестами' }))[0]);
    expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Задача');
    expect(window.location.pathname).toBe('/tasks/be-02');
    // раздел меню остаётся подсвеченным на детальном экране
    expect(within(nav).getByRole('button', { name: 'Задачи' })).toHaveAttribute('aria-current', 'page');
    // чат смонтирован всё время — переписка не теряется при переходах
    expect(screen.getByLabelText('Чат с AI-buddy')).not.toBeVisible();

    act(() => {
      window.history.back();
    });
    await waitFor(() => expect(window.location.pathname).toBe('/tasks'));
    expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Задачи');
  });

  it('«Выйти» в меню — экран входа', async () => {
    window.history.replaceState(null, '', '/desk');
    mockFetch([
      { match: '/api/v1/me', body: { id: 'intern-1', name: 'Стажёр Backend', role: 'intern', team_id: 'backend' } },
      { match: '/api/v1/reminders', body: [] },
    ]);
    renderWithQuery(<App />);
    await userEvent.click(await screen.findByRole('button', { name: 'Выйти' }));
    expect(await screen.findByRole('heading', { name: 'Войти в демо' })).toBeInTheDocument();
    useChatStore.getState().setUser('intern.backend');
  });
});
