import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DraftCard } from './DraftCard';
import { mockFetch, renderWithQuery } from '../test/utils';

afterEach(() => vi.unstubAllGlobals());

const absence = (status: string, date_to = '2026-11-21') => ({
  id: 'ABS-000001',
  date_from: '2026-11-20',
  date_to,
  status,
});

describe('DraftCard', () => {
  it('без дат просит указать дату и не даёт отправить', () => {
    renderWithQuery(<DraftCard args={{ reason: 'personal' }} text="На какую дату?" />);
    expect(screen.getByText('На какую дату?')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Отправить заявку' })).not.toBeInTheDocument();
  });

  it('заявка создаётся только после подтверждения: черновик → confirm → карточка', async () => {
    const spy = mockFetch([
      { method: 'POST', match: '/api/v1/actions', status: 201, body: absence('draft') },
      { method: 'POST', match: '/api/v1/actions/ABS-000001/confirm', body: absence('pending') },
    ]);
    renderWithQuery(<DraftCard args={{ date_from: '2026-11-20', date_to: '2026-11-21' }} />);
    expect(screen.getByText('20.11.2026')).toBeInTheDocument();
    expect(screen.getByText('21.11.2026')).toBeInTheDocument();
    expect(spy).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole('button', { name: 'Отправить заявку' }));
    expect(await screen.findByText(/Заявка ABS-000001 отправлена/)).toBeInTheDocument();
    expect(screen.getByText('ждёт согласования')).toBeInTheDocument();
    expect(screen.getByText(/20.11.2026 – 21.11.2026/)).toBeInTheDocument();
    expect(spy).toHaveBeenCalledTimes(2);
    expect(JSON.parse(spy.mock.calls[0][1]?.body as string)).toMatchObject({
      type: 'absence',
      date_from: '2026-11-20',
      date_to: '2026-11-21',
    });
  });

  it('однодневный отгул: date_to берётся из date_from', async () => {
    const spy = mockFetch([
      { method: 'POST', match: '/api/v1/actions', status: 201, body: absence('draft', '2026-11-20') },
      { method: 'POST', match: '/api/v1/actions/ABS-000001/confirm', body: absence('pending', '2026-11-20') },
    ]);
    renderWithQuery(<DraftCard args={{ date_from: '2026-11-20' }} />);
    await userEvent.click(screen.getByRole('button', { name: 'Отправить заявку' }));
    await screen.findByText(/отправлена/);
    expect(JSON.parse(spy.mock.calls[0][1]?.body as string).date_to).toBe('2026-11-20');
  });

  it('ошибка отправки показывается пользователю', async () => {
    mockFetch([
      { method: 'POST', match: '/api/v1/actions', status: 400, body: { error: { code: 'bad_request', message: 'даты некорректны' } } },
    ]);
    renderWithQuery(<DraftCard args={{ date_from: '2026-11-20' }} />);
    await userEvent.click(screen.getByRole('button', { name: 'Отправить заявку' }));
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('даты некорректны'));
  });

  it('черновик можно отменить', async () => {
    renderWithQuery(<DraftCard args={{ date_from: '2026-11-20' }} />);
    await userEvent.click(screen.getByRole('button', { name: 'Отменить' }));
    expect(screen.getByText('Черновик отменён.')).toBeInTheDocument();
  });

  it('черновик уже сохранён бэком (action_id): только confirm, без второго POST /actions', async () => {
    const spy = mockFetch([
      { method: 'POST', match: '/api/v1/actions/ABS-000001/confirm', body: absence('pending', '2026-11-20') },
    ]);
    renderWithQuery(<DraftCard args={{ action_id: 'ABS-000001', date_from: '2026-11-20' }} />);
    await userEvent.click(screen.getByRole('button', { name: 'Отправить заявку' }));
    expect(await screen.findByText(/Заявка ABS-000001 отправлена/)).toBeInTheDocument();
    expect(spy).toHaveBeenCalledTimes(1);
    expect(String(spy.mock.calls[0][0])).toContain('/confirm');
  });

  it('заявка на доступ: свой заголовок, confirm по action_id, карточка с ресурсом', async () => {
    const spy = mockFetch([
      {
        method: 'POST',
        match: '/api/v1/actions/REQ-000001/confirm',
        body: { id: 'REQ-000001', type: 'access', resource: 'gitlab', status: 'pending' },
      },
    ]);
    renderWithQuery(<DraftCard action="access" args={{ action_id: 'REQ-000001', resource: 'gitlab' }} />);
    expect(screen.getByText(/Черновик заявки на доступ к gitlab/)).toBeInTheDocument();
    expect(screen.queryByText(/Даты:/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Отправить заявку' }));
    expect(await screen.findByText(/Доступ к gitlab/)).toBeInTheDocument();
    expect(screen.queryByText(/предупредить наставника/)).not.toBeInTheDocument();
    expect(spy).toHaveBeenCalledTimes(1);
  });

  it('доступ без action_id: кнопки нет, понятное сообщение (POST /actions умеет только отгул)', () => {
    renderWithQuery(<DraftCard action="access" args={{}} />);
    expect(screen.getByText('📝 Черновик заявки на доступ')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Отправить заявку' })).not.toBeInTheDocument();
    expect(screen.getByText(/не вернул номер черновика/)).toBeInTheDocument();
  });

  it('доступ: ресурс берётся из args, если бэк его не вернул', async () => {
    mockFetch([
      { method: 'POST', match: '/api/v1/actions/REQ-2/confirm', body: { id: 'REQ-2', type: 'access', status: 'pending' } },
    ]);
    renderWithQuery(<DraftCard action="access" args={{ action_id: 'REQ-2', resource: 'jira' }} />);
    await userEvent.click(screen.getByRole('button', { name: 'Отправить заявку' }));
    expect(await screen.findByText(/Доступ к jira/)).toBeInTheDocument();
  });
});
