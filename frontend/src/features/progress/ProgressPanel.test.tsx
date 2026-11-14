import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { closesByApproval, groupByStage, percentDone, ProgressPanel } from './ProgressPanel';
import { mockFetch, renderWithQuery } from '../../test/utils';

afterEach(() => vi.unstubAllGlobals());

const items = [
  { id: 'c1', title: 'Познакомиться с командой', stage: 'week1', done: true },
  { id: 'c2', title: 'Получить доступы', stage: 'week1', done: false },
  { id: 'c3', title: 'Прочитать Playbook', stage: 'week2', done: false },
  { id: 'c4', title: 'Взять первую задачу', stage: 'week2', done: false },
];

describe('ProgressPanel helpers', () => {
  it('группирует по этапам с сохранением порядка', () => {
    const g = groupByStage(items);
    expect(g.map((x) => x.stage)).toEqual(['week1', 'week2']);
    expect(g[1].items).toHaveLength(2);
  });
  it('процент выполнения', () => {
    expect(percentDone(items)).toBe(25);
    expect(percentDone([])).toBe(0);
  });
});

describe('ProgressPanel', () => {
  it('показывает этапы, процент и следующий шаг; отмечает шаг выполненным', async () => {
    const after = items.map((i) => (i.id === 'c2' ? { ...i, done: true } : i));
    const spy = mockFetch([
      { match: '/api/v1/onboarding/progress', body: { items, next_step: 'Получить доступы' } },
      { method: 'PATCH', match: '/api/v1/onboarding/steps/c2', body: { items: after, next_step: 'Прочитать Playbook' } },
    ]);
    renderWithQuery(<ProgressPanel />);
    expect(screen.getByText(/Загружаю/)).toBeInTheDocument();
    expect(await screen.findByText('Выполнено 25%')).toBeInTheDocument();
    expect(screen.getByText('Первая неделя')).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '25');

    await userEvent.click(screen.getByRole('button', { name: 'Отметить «Получить доступы» выполненным' }));
    expect(await screen.findByText('Выполнено 50%')).toBeInTheDocument();
    expect(screen.getByText('Прочитать Playbook', { selector: '.next__title' })).toBeInTheDocument();
    expect(spy.mock.calls.some(([, init]) => init?.method === 'PATCH')).toBe(true);
  });

  it('ошибка изменения шага (чужой шаг → 403) видна пользователю', async () => {
    mockFetch([
      { match: '/api/v1/onboarding/progress', body: { items, next_step: '' } },
      {
        method: 'PATCH',
        match: '/api/v1/onboarding/steps/c3',
        status: 403,
        body: { error: { code: 'forbidden', message: 'Нет прав на этот шаг' } },
      },
    ]);
    renderWithQuery(<ProgressPanel />);
    await userEvent.click(await screen.findByRole('button', { name: /Прочитать Playbook/ }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Нет прав на этот шаг');
  });

  it('ошибка загрузки с кнопкой «Повторить»', async () => {
    const spy = mockFetch([]); // всё → 404
    renderWithQuery(<ProgressPanel />);
    expect(await screen.findByText(/Не удалось загрузить прогресс/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(2));
  });

  it('бэк v2: percent_done от сервера и шаг «ждёт заявку»', async () => {
    const v2 = items.map((i) =>
      i.id === 'c2' ? { ...i, status: 'waiting', related_action_id: 'REQ-000001' } : { ...i, status: i.done ? 'done' : 'todo' },
    );
    mockFetch([{ match: '/api/v1/onboarding/progress', body: { items: v2, next_step: '', percent_done: 33.4 } }]);
    renderWithQuery(<ProgressPanel />);
    expect(await screen.findByText('Выполнено 33%')).toBeInTheDocument();
    const badge = screen.getByText('ждёт согласования');
    expect(badge).toHaveAttribute('title', 'Заявка REQ-000001');
    expect(screen.getByText('закроется после согласования руководителем')).toBeInTheDocument();
    // шаг, который ждёт согласования, нельзя закрыть вручную в обход руководителя
    expect(screen.queryByRole('button', { name: 'Отметить «Получить доступы» выполненным' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Отметить «Прочитать Playbook» выполненным' })).toBeInTheDocument();
  });

  it('шаг kind=access без заявки: вместо «готово» — подсказка запросить доступ', async () => {
    const v2 = items.map((i) => (i.id === 'c2' ? { ...i, status: 'todo', kind: 'access' } : i));
    mockFetch([{ match: '/api/v1/onboarding/progress', body: { items: v2, next_step: '' } }]);
    renderWithQuery(<ProgressPanel />);
    expect(await screen.findByText(/запросите доступ в чате/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Отметить «Получить доступы» выполненным' })).not.toBeInTheDocument();
    expect(closesByApproval({ id: 'x', title: 't', stage: 's', done: false, status: 'waiting' })).toBe(true);
    expect(closesByApproval({ id: 'x', title: 't', stage: 's', done: false, status: 'todo' })).toBe(false);
  });
});
