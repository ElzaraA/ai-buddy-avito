import { useChatStore } from './chat';

const answer = { type: 'answer' as const, text: 'ok' };

beforeEach(() => useChatStore.getState().setUser('intern.backend'));

describe('chat store', () => {
  it('pending → done', () => {
    const s = useChatStore.getState();
    s.addUser('u1', 'вопрос');
    s.addPending('a1');
    useChatStore.getState().resolve('a1', answer);
    const msgs = useChatStore.getState().messages;
    expect(msgs).toHaveLength(2);
    expect(msgs[1]).toMatchObject({ status: 'done', response: answer });
  });

  it('ответ, пришедший раньше сообщения, не теряется (быстрый агент)', () => {
    useChatStore.getState().resolve('a1', answer);
    expect(useChatStore.getState().messages).toHaveLength(0);
    useChatStore.getState().addPending('a1');
    expect(useChatStore.getState().messages[0]).toMatchObject({ status: 'done' });
    expect(useChatStore.getState().early).toEqual({});
  });

  it('ранняя ошибка тоже применяется', () => {
    useChatStore.getState().fail('a2', 'сбой');
    useChatStore.getState().addPending('a2');
    expect(useChatStore.getState().messages[0]).toMatchObject({ status: 'failed', error: 'сбой' });
  });

  it('повторное завершение не перетирает результат (таймаут после ответа)', () => {
    useChatStore.getState().addPending('a3');
    useChatStore.getState().resolve('a3', answer);
    useChatStore.getState().fail('a3', 'таймаут');
    expect(useChatStore.getState().messages[0]).toMatchObject({ status: 'done' });
  });

  it('смена пользователя и reset очищают диалог', () => {
    const s = useChatStore.getState();
    s.setSession('s1');
    s.addUser('u1', 'x');
    s.setUser('intern.other');
    expect(useChatStore.getState()).toMatchObject({ user: 'intern.other', sessionId: null, messages: [] });
    useChatStore.getState().addUser('u2', 'y');
    useChatStore.getState().reset();
    expect(useChatStore.getState().messages).toEqual([]);
  });
});
