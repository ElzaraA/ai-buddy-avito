import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { actionTitle, decisionComment, MyActions } from './MyActions';
import { mockFetch, renderWithQuery } from '../../test/utils';

afterEach(() => vi.unstubAllGlobals());

const pending = { id: 'ABS-000002', type: 'absence' as const, date_from: '2030-01-15', date_to: '2030-01-15', status: 'pending' };
const approved = {
  id: 'REQ-000001',
  type: 'access' as const,
  resource: 'gitlab',
  date_from: '',
  date_to: '',
  status: 'approved',
  history: [
    { by: 'intern-1', status: 'draft' },
    { by: 'intern-1', status: 'pending' },
    { by: 'lead-1', status: 'approved', comment: 'Выдали' },
  ],
};
const draft = { id: 'ABS-000003', type: 'absence' as const, date_from: '2030-02-01', date_to: '2030-02-02', status: 'draft' };

describe('MyActions helpers', () => {
  it('заголовок заявки по типу', () => {
    expect(actionTitle(pending)).toBe('Отгул 15.01.2030');
    expect(actionTitle(draft)).toBe('Отгул 01.02.2030 – 02.02.2030');
    expect(actionTitle(approved)).toBe('Доступ к GitLab');
    expect(actionTitle({ ...approved, resource: undefined })).toBe('Доступ к системе');
  });
  it('комментарий руководителя берётся из последнего решения', () => {
    expect(decisionComment(approved)).toBe('Выдали');
    expect(decisionComment(pending)).toBe('');
  });
});

describe('MyActions', () => {
  it('показывает отправленные заявки со статусом и комментарием; черновики — отдельным блоком', async () => {
    mockFetch([{ match: '/api/v1/actions', body: [pending, approved, draft] }]);
    renderWithQuery(<MyActions />);
    expect(await screen.findByText('Отправленные')).toBeInTheDocument();
    expect(screen.getByText('ждёт согласования')).toBeInTheDocument();
    expect(screen.getByText('согласована')).toBeInTheDocument();
    expect(screen.getByText(/«Выдали»/)).toBeInTheDocument();
    const drafts = screen.getByLabelText('Ждут вашего подтверждения');
    expect(drafts).toHaveTextContent('ABS-000003');
    expect(drafts).toHaveTextContent(/внешнего агента/);
    expect(screen.getByLabelText('Отправленные')).not.toHaveTextContent('ABS-000003');
    // отменить можно только ожидающую заявку; у черновика — «отправить» и «отменить»
    expect(screen.getAllByRole('button')).toHaveLength(3);
  });

  it('черновик от внешнего агента (MCP) подтверждается в интерфейсе', async () => {
    const spy = mockFetch([
      { match: '/api/v1/actions', body: [draft] },
      { method: 'POST', match: '/api/v1/actions/ABS-000003/confirm', body: { ...draft, status: 'pending' } },
    ]);
    renderWithQuery(<MyActions />);
    await userEvent.click(await screen.findByRole('button', { name: 'Отправить заявку ABS-000003' }));
    await waitFor(() => {
      const urls = spy.mock.calls.map(([u]) => String(u));
      expect(urls.some((u) => u.endsWith('/ABS-000003/confirm'))).toBe(true);
      expect(urls.filter((u) => u.endsWith('/api/v1/actions')).length).toBeGreaterThan(1);
    });
  });

  it('отменённые заявки и черновики не попадают в «Отправленные» — отдельный приглушённый блок', async () => {
    const cancelledDraft = { ...draft, id: 'ABS-000009', status: 'cancelled' };
    mockFetch([{ match: '/api/v1/actions', body: [pending, cancelledDraft] }]);
    renderWithQuery(<MyActions />);
    const sent = await screen.findByLabelText('Отправленные');
    expect(sent).toHaveTextContent('ABS-000002');
    expect(sent).not.toHaveTextContent('ABS-000009');
    const cancelled = screen.getByLabelText('Отменённые');
    expect(cancelled).toHaveClass('block--muted');
    expect(cancelled).toHaveTextContent('ABS-000009');
    expect(cancelled).toHaveTextContent('отменена');
    // у отменённых нет кнопок действий
    expect(within(cancelled).queryByRole('button')).not.toBeInTheDocument();
  });

  it('только отменённые — блока «Отправленные» нет', async () => {
    mockFetch([{ match: '/api/v1/actions', body: [{ ...draft, status: 'cancelled' }] }]);
    renderWithQuery(<MyActions />);
    expect(await screen.findByLabelText('Отменённые')).toBeInTheDocument();
    expect(screen.queryByLabelText('Отправленные')).not.toBeInTheDocument();
  });

  it('кнопки честно называются «Отменить черновик» / «Отменить заявку» — без «Удалить»', async () => {
    mockFetch([{ match: '/api/v1/actions', body: [pending, draft] }]);
    renderWithQuery(<MyActions />);
    expect(await screen.findByRole('button', { name: 'Отменить черновик ABS-000003' })).toHaveTextContent(
      'Отменить черновик',
    );
    expect(screen.getByRole('button', { name: 'Отменить заявку ABS-000002' })).toHaveTextContent('Отменить заявку');
    expect(screen.queryByText(/Удалить/)).not.toBeInTheDocument();
  });

  it('черновик можно отменить: вызывается cancel на сервере', async () => {
    const spy = mockFetch([
      { match: '/api/v1/actions', body: [draft] },
      { method: 'POST', match: '/api/v1/actions/ABS-000003/cancel', body: { ...draft, status: 'cancelled' } },
    ]);
    renderWithQuery(<MyActions />);
    await userEvent.click(await screen.findByRole('button', { name: 'Отменить черновик ABS-000003' }));
    await waitFor(() => expect(spy.mock.calls.some(([u]) => String(u).endsWith('/ABS-000003/cancel'))).toBe(true));
  });

  it('отмена заявки вызывает cancel и перечитывает список', async () => {
    const spy = mockFetch([
      { match: '/api/v1/actions', body: [pending] },
      { method: 'POST', match: '/api/v1/actions/ABS-000002/cancel', body: { ...pending, status: 'cancelled' } },
    ]);
    renderWithQuery(<MyActions />);
    await userEvent.click(await screen.findByRole('button', { name: 'Отменить заявку ABS-000002' }));
    await waitFor(() => {
      const urls = spy.mock.calls.map(([u]) => String(u));
      expect(urls.some((u) => u.endsWith('/cancel'))).toBe(true);
      expect(urls.filter((u) => u.endsWith('/api/v1/actions')).length).toBeGreaterThan(1);
    });
  });

  it('ошибка отмены (409) — понятный текст и перечитывание списка; без заявок блок не рисуется', async () => {
    const spy = mockFetch([
      { match: '/api/v1/actions', body: [pending] },
      { method: 'POST', match: '/api/v1/actions/ABS-000002/cancel', status: 409, body: { error: { code: 'conflict', message: 'action is already decided' } } },
    ]);
    const first = renderWithQuery(<MyActions />);
    await userEvent.click(await screen.findByRole('button', { name: 'Отменить заявку ABS-000002' }));
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Заявка уже обработана, список обновлён');
    expect(alert).not.toHaveTextContent('action is already decided');
    await waitFor(() =>
      expect(spy.mock.calls.filter(([u]) => String(u).endsWith('/api/v1/actions')).length).toBeGreaterThan(1),
    );
    first.unmount();

    mockFetch([{ match: '/api/v1/actions', body: null }]);
    const { container } = renderWithQuery(<MyActions />);
    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });
});
