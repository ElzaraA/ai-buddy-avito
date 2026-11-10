import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { groupByStage, percentDone, ProgressPanel } from './ProgressPanel';
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
});
