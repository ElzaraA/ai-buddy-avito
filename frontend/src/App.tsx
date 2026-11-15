import { useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api, setDemoUser } from './api/client';
import { ChatPanel } from './features/chat/ChatPanel';
import { ProgressPanel } from './features/progress/ProgressPanel';
import { TeamPanel } from './features/team/TeamPanel';
import { DEMO_USERS, useChatStore } from './store/chat';

export function App() {
  const user = useChatStore((s) => s.user);
  const setUser = useChatStore((s) => s.setUser);

  // заголовок X-Demo-User должен смениться до любых запросов
  setDemoUser(user);
  useEffect(() => setDemoUser(user), [user]);

  const me = useQuery({ queryKey: ['me', user], queryFn: api.me });
  const role = me.data?.role;
  const isTeamView = role === 'buddy' || role === 'lead';

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <span className="brand__name">AI-buddy</span>
          <span className="brand__tagline">помощник стажёра</span>
        </div>

        {me.data && (
          <div className="me">
            <span className="me__name">{me.data.name}</span>
            <span className="me__team">команда {me.data.team_id}</span>
          </div>
        )}

        <label className="user-switch">
          <span>Войти как</span>
          <select value={user} onChange={(e) => setUser(e.target.value)} aria-label="Тестовый пользователь">
            {DEMO_USERS.map((u) => (
              <option key={u.login} value={u.login}>
                {u.label}
              </option>
            ))}
          </select>
        </label>
      </header>

      <main className="layout">
        <ChatPanel key={`chat-${user}`} staff={isTeamView} />
        {/* наставник и руководитель видят кабинет команды, стажёр — свой план адаптации */}
        {isTeamView ? <TeamPanel key={`team-${user}`} role={role} /> : <ProgressPanel />}
      </main>

      <footer className="footer">
        Прототип на синтетических данных. Инструкции тестовые и не являются регламентами Авито.
      </footer>
    </div>
  );
}
