import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api, setDemoUser } from './api/client';
import { ChatPanel, PROBLEM_PREFIX } from './features/chat/ChatPanel';
import { ActionsView } from './features/home/ActionsView';
import { Desk } from './features/home/Desk';
import { Skills } from './features/home/Skills';
import { ProgressPanel } from './features/progress/ProgressPanel';
import { Sidebar } from './features/shell/Sidebar';
import { defaultView, viewsFor, type ViewId } from './features/shell/views';
import { TeamPanel } from './features/team/TeamPanel';
import { useChatStore } from './store/chat';

const SUBTITLES: Partial<Record<ViewId, string>> = {
  desk: 'Начните с карточки или спросите Бадди своими словами',
  plan: 'Чек-лист адаптации: шаги с заявками закрываются сами после согласования',
  actions: 'Черновики, которые ждут отправки, и решения руководителя',
  skills: 'Готовые вопросы: выберите — и Бадди ответит в чате',
  team: 'Кто просит помощи, что ждёт решения и чего нет в базе знаний',
  chat: 'Проверьте, как Бадди отвечает стажёрам',
};

export function App() {
  const user = useChatStore((s) => s.user);
  const prefill = useChatStore((s) => s.prefill);
  const ask = useChatStore((s) => s.ask);

  // заголовок X-Demo-User должен смениться до любых запросов
  setDemoUser(user);
  useEffect(() => setDemoUser(user), [user]);

  const me = useQuery({ queryKey: ['me', user], queryFn: api.me });
  const role = me.data?.role;
  const isTeamView = role === 'buddy' || role === 'lead';

  // раздел; при смене учётки возвращаемся на стартовый (чат стажёра или кабинет команды)
  const [view, setView] = useState<ViewId | null>(null);
  useEffect(() => setView(null), [user]);
  const views = viewsFor(isTeamView);
  const active: ViewId = view && views.some((v) => v.id === view) ? view : defaultView(isTeamView);
  const current = views.find((v) => v.id === active) ?? views[0];
  const chatView: ViewId = isTeamView ? 'chat' : 'desk';
  const chatVisible = active === chatView;

  const openChat = (text = '') => {
    setView(chatView);
    prefill(text);
  };
  const report = () => openChat(PROBLEM_PREFIX);
  const askBuddy = (q: string) => {
    setView(chatView);
    ask(q);
  };

  return (
    <div className="app">
      <div className="shell">
        <Sidebar
          views={views}
          view={active}
          staff={isTeamView}
          me={me.data}
          onNavigate={setView}
          onWrite={() => openChat()}
          onReport={report}
        />

        <main className="workspace">
          {/* Навигация — только боковое меню: вкладки над контентом дублировали его один в один */}
          <header className="workspace__head">
            <div className="workspace__titles">
              <h1 className="workspace__title">{current.title}</h1>
              <p className="workspace__subtitle">{SUBTITLES[active]}</p>
            </div>
          </header>

          <div className={`workspace__body${chatVisible ? ' workspace__body--chat' : ''}`}>
            {!isTeamView && (
              <div hidden={active !== 'desk'}>
                <Desk onOpenPlan={() => setView('plan')} onOpenActions={() => setView('actions')} onReport={report} />
              </div>
            )}

            {/* чат всегда смонтирован: переписка и поток ответов не теряются при переходах по разделам */}
            <div className={`chat-card${isTeamView ? ' chat-card--full' : ''}`} hidden={!chatVisible}>
              <ChatPanel key={`chat-${user}`} staff={isTeamView} />
            </div>

            {isTeamView ? (
              <div hidden={active !== 'team'}>
                <TeamPanel key={`team-${user}`} role={role} />
              </div>
            ) : (
              <>
                <div hidden={active !== 'plan'}>
                  <ProgressPanel withActions={false} />
                </div>
                <div hidden={active !== 'actions'}>
                  <ActionsView onAsk={askBuddy} />
                </div>
                <div hidden={active !== 'skills'}>
                  <Skills onAsk={askBuddy} />
                </div>
              </>
            )}
          </div>

          <footer className="footer">
            Прототип на синтетических данных. Инструкции тестовые и не являются регламентами Авито.
          </footer>
        </main>
      </div>
    </div>
  );
}
