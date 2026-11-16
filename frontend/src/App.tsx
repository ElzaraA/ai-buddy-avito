import { useEffect, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api, setDemoUser } from './api/client';
import { ChatPanel, PROBLEM_PREFIX } from './features/chat/ChatPanel';
import { ActionsView } from './features/home/ActionsView';
import { Desk } from './features/home/Desk';
import { ProfileView } from './features/home/ProfileView';
import { Skills } from './features/home/Skills';
import { ProgressPanel } from './features/progress/ProgressPanel';
import { StepDetails } from './features/progress/StepDetails';
import { ScheduleView } from './features/schedule/ScheduleView';
import { useReminders } from './features/schedule/useReminders';
import { Sidebar } from './features/shell/Sidebar';
import { defaultView, matchPath, pathForView, viewsFor, type ViewId } from './features/shell/views';
import { TeamPanel } from './features/team/TeamPanel';
import { guessRole, useChatStore } from './store/chat';

const SUBTITLES: Partial<Record<ViewId, string>> = {
  desk: 'Начните с карточки или спросите Бадди своими словами',
  plan: 'Чек-лист адаптации: шаги с заявками закрываются сами после согласования',
  schedule: 'Встречи команды и ваши созвоны; о каждой Бадди напомнит в чате',
  actions: 'Черновики, которые ждут отправки, и решения руководителя',
  skills: 'Готовые вопросы: выберите — и Бадди ответит в чате',
  team: 'Кто просит помощи, что ждёт решения и чего нет в базе знаний',
  chat: 'Проверьте, как Бадди отвечает стажёрам',
};

export function App() {
  const user = useChatStore((s) => s.user);
  const prefill = useChatStore((s) => s.prefill);
  const ask = useChatStore((s) => s.ask);
  const previousUser = useRef(user);

  // заголовок X-Demo-User должен смениться до любых запросов
  setDemoUser(user);
  useEffect(() => setDemoUser(user), [user]);

  const me = useQuery({ queryKey: ['me', user], queryFn: api.me });
  useReminders();
  const unreadReminders = useChatStore((s) => s.unreadReminders);
  const readReminders = useChatStore((s) => s.readReminders);
  // пока /me грузится после смены учётки — роль по логину, чтобы не мелькал чужой экран
  const role = me.data?.role ?? guessRole(user);
  const isTeamView = role === 'buddy' || role === 'lead';

  // раздел; при смене учётки возвращаемся на стартовый (чат стажёра или кабинет команды)
  const [pathname, setPathname] = useState(() => window.location.pathname);
  // растёт после «Сбросить демо»: ключ для перемонтирования чата, стола и кабинета
  const [epoch, setEpoch] = useState(0);
  const views = viewsFor(isTeamView);
  const route = matchPath(pathname, isTeamView);
  const active: ViewId = route.kind === 'view' ? route.view : route.kind === 'step' ? 'plan' : defaultView(isTeamView);
  const forbidden = route.kind === 'view'
    ? !views.some((v) => v.id === route.view)
    : route.kind === 'step' && isTeamView;
  const current = views.find((v) => v.id === active) ?? views[0];
  const chatView: ViewId = isTeamView ? 'chat' : 'desk';
  const chatVisible = route.kind === 'view' && (active === chatView || active === 'chat');

  const navigate = (next: ViewId, replace = false) => {
    const path = pathForView(next);
    if (window.location.pathname !== path) {
      if (replace) window.history.replaceState(null, '', path);
      else window.history.pushState(null, '', path);
    }
    setPathname(path);
  };

  useEffect(() => {
    const onPopState = () => setPathname(window.location.pathname);
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  useEffect(() => {
    if (previousUser.current === user) return;
    previousUser.current = user;
    const target = pathForView(defaultView(isTeamView));
    if (pathname === target) return;
    window.history.replaceState(null, '', target);
    setPathname(target);
  }, [user]);
  // напоминание прочитано, как только чат на экране
  useEffect(() => {
    if (chatVisible && unreadReminders > 0) readReminders();
  }, [chatVisible, unreadReminders, readReminders]);

  // смена раздела или учётки — рабочая область с начала: после кабинета руководителя
  // стол стажёра не должен открываться прокрученным к чату
  const workspaceRef = useRef<HTMLElement>(null);
  useEffect(() => {
    const ws = workspaceRef.current;
    if (!ws) return;
    ws.scrollTop = 0;
    if (window.scrollY > 0) window.scrollTo(0, 0);
    // «Сообщить наставнику» ставит фокус в поле ввода — оно должно остаться на экране
    const focused = document.activeElement;
    if (focused instanceof HTMLElement && focused !== document.body && ws.contains(focused)) {
      focused.scrollIntoView?.({ block: 'nearest' });
    }
  }, [active, user, epoch]);

  const openChat = (text = '') => {
    navigate(chatView);
    prefill(text);
  };
  const report = () => openChat(PROBLEM_PREFIX);
  const askBuddy = (q: string) => {
    navigate(chatView);
    ask(q);
  };
  // «Сбросить демо»: стартовый раздел и заново смонтированные чат и кабинет (без набранного текста и черновиков ответов)
  const afterDemoReset = () => {
    navigate(defaultView(isTeamView), true);
    setEpoch((e) => e + 1);
  };

  const openStep = (stepId: string) => {
    const path = `/plan/${encodeURIComponent(stepId)}`;
    window.history.pushState(null, '', path);
    setPathname(path);
  };

  const routeTitle = route.kind === 'not-found'
    ? 'Страница не найдена'
    : forbidden
      ? 'Доступ запрещён'
      : route.kind === 'step'
        ? 'Шаг адаптации'
        : current.title;

  return (
    <div className="app">
      <div className="shell">
        <Sidebar
          views={views}
          view={active}
          staff={isTeamView}
          me={me.data}
          onNavigate={navigate}
          badges={chatVisible || unreadReminders === 0 ? {} : { [chatView]: unreadReminders }}
          onWrite={() => openChat()}
          onReport={report}
          onDemoReset={afterDemoReset}
        />

        <main className="workspace" ref={workspaceRef}>
          {/* Навигация — только боковое меню: вкладки над контентом дублировали его один в один */}
          <header className="workspace__head">
            <div className="workspace__titles">
              <h1 className="workspace__title">{routeTitle}</h1>
              <p className="workspace__subtitle">{route.kind === 'view' && !forbidden ? SUBTITLES[active] : ''}</p>
            </div>
          </header>

          <div
            className={`workspace__body${chatVisible ? ' workspace__body--chat' : ''}${
              active === 'desk' && !isTeamView ? ' workspace__body--desk' : ''
            }`}
          >
            {route.kind === 'not-found' ? (
              <section className="sheet sheet--state" role="status">
                <p>Проверьте адрес или вернитесь к рабочему разделу.</p>
                <button className="btn btn--link" onClick={() => navigate(defaultView(isTeamView))}>На главную</button>
              </section>
            ) : forbidden ? (
              <section className="sheet sheet--state" role="alert">
                <p>Этот раздел недоступен для текущей роли.</p>
                <button className="btn btn--link" onClick={() => navigate(defaultView(isTeamView))}>Вернуться в свой кабинет</button>
              </section>
            ) : route.kind === 'step' ? (
              <StepDetails stepId={route.stepId} onBack={() => navigate('plan')} />
            ) : active === 'profile' ? (
              <ProfileView user={me.data} isLoading={me.isLoading} isError={me.isError} onRetry={() => void me.refetch()} />
            ) : <>
            {!isTeamView && (
              <div className="desk-slot" hidden={active !== 'desk'}>
                <Desk key={`desk-${epoch}`} onOpenPlan={() => navigate('plan')} onOpenActions={() => navigate('actions')} onReport={report} />
              </div>
            )}

            {/* чат всегда смонтирован: переписка и поток ответов не теряются при переходах по разделам */}
            <div className={`chat-card${isTeamView ? ' chat-card--full' : ''}`} hidden={!chatVisible}>
              <ChatPanel key={`chat-${user}-${epoch}`} staff={isTeamView} />
            </div>

            <div hidden={active !== 'schedule'}>
              {active === 'schedule' && <ScheduleView key={`schedule-${user}`} />}
            </div>

            {isTeamView ? (
              <div hidden={active !== 'team'}>
                <TeamPanel key={`team-${user}-${epoch}`} role={role} />
              </div>
            ) : (
              <>
                <div hidden={active !== 'plan'}>
                  <ProgressPanel withActions={false} onOpenStep={openStep} />
                </div>
                <div hidden={active !== 'actions'}>
                  <ActionsView onAsk={askBuddy} />
                </div>
                <div hidden={active !== 'skills'}>
                  <Skills onAsk={askBuddy} />
                </div>
              </>
            )}
            </>}
          </div>

          <footer className="footer">
            Прототип на синтетических данных. Инструкции тестовые и не являются регламентами Авито.
          </footer>
        </main>
      </div>
    </div>
  );
}
