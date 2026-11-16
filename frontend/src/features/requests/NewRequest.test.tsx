import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { mockFetch, renderWithQuery } from '../../test/utils';
import { NewRequest, todayMsk, validateAbsence } from './NewRequest';

afterEach(() => vi.unstubAllGlobals());

describe('validateAbsence', () => {
  it('те же правила, что на сервере', () => {
    expect(validateAbsence('', '', '2026-11-16')).toMatch(/Укажите даты/);
    expect(validateAbsence('2026-11-15', '2026-11-15', '2026-11-16')).toMatch(/прошла/);
    expect(validateAbsence('2026-11-20', '2026-11-19', '2026-11-16')).toMatch(/раньше/);
    expect(validateAbsence('2026-11-20', '2026-12-31', '2026-11-16')).toMatch(/30 дней/);
    expect(validateAbsence('2026-11-20', '2026-11-21', '2026-11-16')).toBeNull();
    expect(todayMsk(new Date('2026-11-15T22:30:00Z'))).toBe('2026-11-16'); // по Москве уже завтра
  });
});

describe('NewRequest', () => {
  it('доступ: черновик на сервере → подтверждение → отправлено', async () => {
    const fetch = mockFetch([
      { method: 'POST', match: '/api/v1/actions', status: 201, body: { id: 'REQ-000001', type: 'access', resource: 'jira', status: 'draft' } },
      { method: 'POST', match: '/api/v1/actions/REQ-000001/confirm', body: { id: 'REQ-000001', type: 'access', resource: 'jira', status: 'pending' } },
    ]);
    renderWithQuery(<NewRequest type="access" />);
    await userEvent.selectOptions(screen.getByLabelText('Система'), 'jira');
    await userEvent.type(screen.getByLabelText(/Причина/), 'задачи спринта');
    await userEvent.click(screen.getByRole('button', { name: 'Подготовить заявку' }));
    expect(await screen.findByText('Черновик заявки на доступ к Jira')).toBeInTheDocument();
    expect(screen.getByText(/пока вы не нажали «Отправить», она никуда не уходит/)).toBeInTheDocument();
    const post = fetch.mock.calls.find(([u, i]) => String(u).endsWith('/api/v1/actions') && i?.method === 'POST');
    expect(JSON.parse(String(post?.[1]?.body))).toEqual({ type: 'access', resource: 'jira', reason: 'задачи спринта' });
    // ничего не ушло руководителю, пока стажёр не подтвердил
    expect(fetch.mock.calls.some(([u]) => String(u).endsWith('/confirm'))).toBe(false);
    await userEvent.click(screen.getByRole('button', { name: 'Отправить заявку' }));
    expect(await screen.findByText('Заявка REQ-000001 отправлена')).toBeInTheDocument();
  });

  it('отгул: проверка дат до отправки и ошибка сервера', async () => {
    const fetch = mockFetch([
      {
        method: 'POST',
        match: '/api/v1/actions',
        status: 429,
        body: { error: { code: 'too_many_drafts', message: 'Слишком много неподтверждённых черновиков' } },
      },
    ]);
    renderWithQuery(<NewRequest type="absence" />);
    const from = screen.getByLabelText('С');
    const to = screen.getByLabelText('По');
    await userEvent.clear(to);
    await userEvent.type(to, '2020-01-01');
    await userEvent.click(screen.getByRole('button', { name: 'Подготовить заявку' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/раньше даты начала/);
    expect(fetch).not.toHaveBeenCalled();

    await userEvent.clear(to);
    await userEvent.type(to, (from as HTMLInputElement).value);
    await userEvent.click(screen.getByRole('button', { name: 'Подготовить заявку' }));
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Слишком много неподтверждённых черновиков'));
  });

  it('вкладки типа заявки — ссылки с адресом', () => {
    renderWithQuery(<NewRequest type="absence" />);
    expect(screen.getByRole('link', { name: 'Доступ к системе' })).toHaveAttribute('href', '/actions/new?type=access');
    expect(screen.getByRole('link', { name: 'Отгул' })).toHaveAttribute('aria-current', 'page');
  });
});
