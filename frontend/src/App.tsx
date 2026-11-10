import { useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api, setDemoUser } from './api/client';
import { ChatPanel } from './features/chat/ChatPanel';
import { ProgressPanel } from './features/progress/ProgressPanel';
import { DEMO_USERS, useChatStore } from './store/chat';

export function App() {
  const user = useChatStore((s) => s.user);
  const setUser = useChatStore((s) => s.setUser);

  // заголовок X-Demo-User должен смениться до любых запросов
  setDemoUser(user);
  useEffect(() => setDemoUser(user), [user]);

  const me = useQuery({ queryKey: ['me', user], queryFn: api.me });

  return (
    <div className="app">
      <header className="topbar">
        <div className="logo">
          AI-buddy <span className="muted">· помощник стажёра</span>
        </div>
        <label className="user-switch">
          <span className="muted">Тестовый пользователь:</span>
          <select value={user} onChange={(e) => setUser(e.target.value)} aria-label="Тестовый пользователь">
            {DEMO_USERS.map((u) => (
              <option key={u.login} value={u.login}>
                {u.label}
              </option>
            ))}
          </select>
        </label>
        {me.data && (
          <div className="me">
            {me.data.name} · {me.data.team_id}
          </div>
        )}
      </header>

      <main className="layout">
        <ChatPanel key={`chat-${user}`} />
        <ProgressPanel />
      </main>

      <footer className="footer muted">
        Прототип. Данные синтетические, инструкции тестовые и не являются регламентами Авито.
      </footer>
    </div>
  );
}
