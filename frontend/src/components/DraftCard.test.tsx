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
});
