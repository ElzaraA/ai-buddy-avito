import { screen, waitFor } from '@testing-library/react';
import { MySignals, signalState, signalSummary } from './MySignals';
import { Desk } from '../home/Desk';
import { useChatStore } from '../../store/chat';
import { mockFetch, renderWithQuery } from '../../test/utils';

beforeEach(() => useChatStore.getState().setUser('intern.backend'));
afterEach(() => vi.unstubAllGlobals());

const waiting = { id: 'SIG-1', text: 'Передай наставнику: VPN не работает', topic: 'tech', at: '2026-11-15T09:00:00Z', done: false };
const answered = {
  id: 'SIG-2',
  text: 'Где Playbook?',
  topic: 'unclear',
  at: '2026-11-14T09:00:00Z',
  done: true,
  reply: 'В Confluence, раздел Onboarding',
  resolved_by: 'mentor-1',
  resolved_at: '2026-11-14T10:00:00Z',
};
const closed = { id: 'SIG-3', text: 'ok', topic: 'other', done: true };

describe('MySignals helpers', () => {
  it('статус: ждёт ответа / наставник ответил / разобрано без ответа', () => {
    expect(signalState({ ...waiting, user_id: '' })).toBe('waiting');
    expect(signalState({ ...answered, user_id: '' })).toBe('answered');
    expect(signalState({ ...closed, user_id: '' })).toBe('closed');
  });
  it('сводка: сколько ждут и самый свежий ответ', () => {
    const later = { ...answered, id: 'SIG-4', reply: 'Новый ответ', resolved_at: '2026-11-15T12:00:00Z', user_id: '' };
    const s = signalSummary([{ ...waiting, user_id: '' }, { ...answered, user_id: '' }, later]);
    expect(s.waiting).toBe(1);
    expect(s.answered?.id).toBe('SIG-4');
    expect(s.text).toBe('ждёт ответа: 1; наставник ответил: «Новый ответ»');
  });
});

describe('MySignals', () => {
  it('«Мои обращения к наставнику»: статус и текст ответа', async () => {
    mockFetch([{ match: '/api/v1/signals/me', body: [waiting, answered, closed] }]);
    renderWithQuery(<MySignals />);
    const block = await screen.findByRole('region', { name: 'Мои обращения к наставнику' });
    expect(block).toHaveTextContent('ждёт ответа');
    expect(block).toHaveTextContent('наставник ответил');
    expect(block).toHaveTextContent('Ответ наставника: В Confluence, раздел Onboarding');
    // заготовку «Передай наставнику:» не повторяем
    expect(block).toHaveTextContent('VPN не работает');
    expect(block).not.toHaveTextContent('Передай наставнику');
  });

  it('старый бэк без /signals/me (404) — блок не рисуется и не опрашивается', async () => {
    const spy = mockFetch([]);
    const { container } = renderWithQuery(<MySignals />);
    await waitFor(() => expect(spy).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
    expect(spy).toHaveBeenCalledTimes(1);
  });

  it('перечитывается периодически: ответ наставника появляется сам', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let body: unknown[] = [waiting];
    const spy = vi.fn(async () => new Response(JSON.stringify(body), { status: 200 }));
    vi.stubGlobal('fetch', spy);
    renderWithQuery(<MySignals />);
    expect(await screen.findByText('ждёт ответа')).toBeInTheDocument();
    body = [{ ...waiting, done: true, reply: 'Перезапусти клиент' }];
    await vi.advanceTimersByTimeAsync(5100);
    expect(await screen.findByText(/Перезапусти клиент/)).toBeInTheDocument();
    vi.useRealTimers();
  });
});

describe('Desk: индикатор на карточке «Нужна помощь?»', () => {
  it('показывает «ждёт ответа» и ответ наставника', async () => {
    mockFetch([
      { match: '/api/v1/onboarding/progress', body: { items: [], next_step: '' } },
      { match: '/api/v1/actions', body: [] },
      { match: '/api/v1/signals/me', body: [waiting, answered] },
    ]);
    const noop = () => {};
    renderWithQuery(<Desk onOpenPlan={noop} onOpenActions={noop} onReport={noop} />);
    expect(await screen.findByText('ждёт ответа: 1')).toBeInTheDocument();
    expect(screen.getByText('Наставник ответил: «В Confluence, раздел Onboarding»')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Нужна помощь\? Сообщить наставнику\. Мои обращения: ждёт ответа: 1/ })).toBeInTheDocument();
  });
});
