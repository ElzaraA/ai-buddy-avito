import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { mockFetch, renderWithQuery } from '../../test/utils';
import { HistoryView } from './HistoryView';

afterEach(() => vi.unstubAllGlobals());

describe('HistoryView', () => {
  it('список диалогов и переписка по клику', async () => {
    mockFetch([
      {
        match: '/api/v1/chat/sessions',
        body: [{ id: 's1', last_at: '2026-11-16T10:00:00Z', first_question: 'как оформить отгул?', messages: 4 }],
      },
      {
        match: '/api/v1/chat/sessions/s1/messages',
        body: [
          { id: 'm1', role: 'user', content: 'как оформить отгул?', status: 'done' },
          { id: 'm2', role: 'assistant', content: '', status: 'done', result: JSON.stringify({ type: 'answer', text: 'Через заявку руководителю.' }) },
          { id: 'm3', role: 'user', content: 'а ещё?', status: 'done' },
          { id: 'm4', role: 'assistant', content: 'Сервер был перезапущен', status: 'failed' },
          { id: 'm5', role: 'assistant', content: 'старый ответ', status: 'done', result: 'не json' },
          { id: 'm6', role: 'assistant', content: '', status: 'pending' },
        ],
      },
    ]);
    renderWithQuery(<HistoryView />);
    const head = await screen.findByRole('button', { name: /как оформить отгул\?/ });
    expect(head).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getByText(/сообщений: 4/)).toBeInTheDocument();
    await userEvent.click(head);
    expect(await screen.findByText('Через заявку руководителю.')).toBeInTheDocument();
    expect(screen.getByText('Сервер был перезапущен')).toBeInTheDocument();
    expect(screen.getByText('старый ответ')).toBeInTheDocument();
    expect(screen.getByText('Ответ ещё готовится…')).toBeInTheDocument();
    await userEvent.click(head);
    expect(screen.queryByText('Через заявку руководителю.')).not.toBeInTheDocument();
  });

  it('пусто и ошибки', async () => {
    mockFetch([{ match: '/api/v1/chat/sessions', body: [] }]);
    const { unmount } = renderWithQuery(<HistoryView />);
    expect(await screen.findByText(/Диалогов пока нет/)).toBeInTheDocument();
    unmount();
    mockFetch([
      { match: '/api/v1/chat/sessions', body: [{ id: 's2', first_question: '', messages: 0 }] },
      { match: '/api/v1/chat/sessions/s2/messages', status: 403, body: { error: { code: 'forbidden', message: 'x' } } },
    ]);
    const second = renderWithQuery(<HistoryView />);
    await userEvent.click(await screen.findByRole('button', { name: /Диалог без вопросов/ }));
    expect(await screen.findByText('Не удалось загрузить переписку.')).toBeInTheDocument();
    second.unmount();
    mockFetch([{ match: '/api/v1/chat/sessions', status: 500, body: {} }]);
    renderWithQuery(<HistoryView />);
    expect(await screen.findByText(/Не удалось загрузить историю/)).toBeInTheDocument();
  });
});
