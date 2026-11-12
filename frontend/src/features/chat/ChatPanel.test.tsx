import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ChatPanel, SUGGESTIONS } from './ChatPanel';
import { AGENT_TIMEOUT_MS } from './useChat';
import { useChatStore } from '../../store/chat';
import { FakeEventSource, mockFetch, renderWithQuery } from '../../test/utils';

let n = 0;
function chatRoutes() {
  return [
    { method: 'POST', match: '/api/v1/chat/sessions', status: 201, body: { id: 's1' } },
    {
      method: 'POST',
      match: '/api/v1/chat/sessions/s1/messages',
      status: 202,
      get body() {
        n += 1;
        return { user_message_id: `u${n}`, assistant_message_id: `a${n}`, status: 'pending' };
      },
    },
  ];
}

beforeEach(() => {
  n = 0;
  FakeEventSource.instances = [];
  vi.stubGlobal('EventSource', FakeEventSource);
  useChatStore.getState().setUser('intern.backend');
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

async function ask(text: string) {
  await userEvent.type(screen.getByLabelText('Сообщение'), text);
  await userEvent.click(screen.getByRole('button', { name: 'Отправить' }));
}

describe('ChatPanel', () => {
  it('пустой чат показывает подсказки, отправка — «думает», затем ответ по SSE', async () => {
    mockFetch(chatRoutes());
    renderWithQuery(<ChatPanel />);
    SUGGESTIONS.forEach((s) => expect(screen.getByText(s)).toBeInTheDocument());

    await ask('Как устроено код-ревью?');
    expect(await screen.findByText('Как устроено код-ревью?')).toBeInTheDocument();
    expect(screen.getByText(/Помощник думает/)).toBeInTheDocument();
    expect(screen.getByLabelText('Сообщение')).toHaveValue('');

    const es = FakeEventSource.last();
    expect(es.url).toBe('/api/v1/chat/sessions/s1/events?user=intern.backend');
    act(() =>
      es.emit('message_done', {
        message_id: 'a1',
        status: 'done',
        result: { type: 'answer', text: 'Ревью делает наставник', sources: [{ id: 'x', source: 'tools.md' }] },
      }),
    );
    expect(await screen.findByText('Ревью делает наставник')).toBeInTheDocument();
    expect(screen.queryByText(/Помощник думает/)).not.toBeInTheDocument();
  });

  it('клик по подсказке отправляет её; сессия создаётся один раз', async () => {
    const spy = mockFetch(chatRoutes());
    renderWithQuery(<ChatPanel />);
    await userEvent.click(screen.getByText(SUGGESTIONS[0]));
    await screen.findByText(/Помощник думает/);
    await ask('ещё вопрос');
    await waitFor(() => expect(screen.getAllByText(/Помощник думает/)).toHaveLength(2));
    const sessionCalls = spy.mock.calls.filter(([u]) => String(u).endsWith('/chat/sessions'));
    expect(sessionCalls).toHaveLength(1);
  });

  it('событие action обновляет прогресс, ошибка агента — понятное сообщение', async () => {
    mockFetch(chatRoutes());
    const { client } = renderWithQuery(<ChatPanel />);
    const invalidate = vi.spyOn(client, 'invalidateQueries');
    await ask('доступ в Jira');
    await ask('второй');
    const es = FakeEventSource.last();
    act(() =>
      es.emit('message_done', {
        message_id: 'a1',
        status: 'done',
        result: { type: 'action', result: { ok: true, message: 'Заявка на доступ создана', data: { request_id: 'REQ-1' } } },
      }),
    );
    expect(await screen.findByText('REQ-1')).toBeInTheDocument();
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['progress'] });

    act(() => es.emit('error', { message_id: 'a2', error: 'LLM недоступна' }));
    expect(await screen.findByText(/LLM недоступна/)).toBeInTheDocument();
  });

  it('message_done со статусом failed и мусорные события', async () => {
    mockFetch(chatRoutes());
    renderWithQuery(<ChatPanel />);
    await ask('вопрос');
    const es = FakeEventSource.last();
    act(() => {
      es.emit('message_done', { bad: true }); // не по схеме — игнор
      es.emit('error'); // обрыв соединения без данных — игнор
      es.emit('error', { nope: 1 });
    });
    expect(screen.getByText(/Помощник думает/)).toBeInTheDocument();
    act(() => es.emit('message_done', { message_id: 'a1', status: 'failed' }));
    expect(await screen.findByText(/не смог ответить/)).toBeInTheDocument();
  });

  it('таймаут: агент не ответил — ошибка, интерфейс не висит', async () => {
    mockFetch(chatRoutes());
    vi.useFakeTimers({ shouldAdvanceTime: true });
    renderWithQuery(<ChatPanel />);
    await ask('долгий вопрос');
    await screen.findByText(/Помощник думает/);
    act(() => vi.advanceTimersByTime(AGENT_TIMEOUT_MS + 10));
    expect(await screen.findByText(/не ответил вовремя/)).toBeInTheDocument();
  });

  it('сбой отправки показывается, пустое сообщение не отправляется', async () => {
    const spy = mockFetch([]); // всё → 404
    renderWithQuery(<ChatPanel />);
    expect(screen.getByRole('button', { name: 'Отправить' })).toBeDisabled();
    await ask('вопрос');
    expect(await screen.findByRole('alert')).toHaveTextContent('/api/v1/chat/sessions');
    expect(spy).toHaveBeenCalledTimes(1);
  });

  it('совместимость: бэк присылает messageId вместо message_id', async () => {
    mockFetch(chatRoutes());
    renderWithQuery(<ChatPanel />);
    await ask('вопрос');
    await ask('второй');
    const es = FakeEventSource.last();
    act(() => es.emit('message_done', { messageId: 'a1', status: 'done', result: { type: 'answer', text: 'Ответ v1' } }));
    expect(await screen.findByText('Ответ v1')).toBeInTheDocument();
    act(() => es.emit('error', { messageId: 'a2', error: 'agent failed' }));
    expect(await screen.findByText(/agent failed/)).toBeInTheDocument();
  });
});
