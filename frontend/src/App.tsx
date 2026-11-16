import { useEffect, useRef, useState, useSyncExternalStore, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ApiError, api, setDemoUser } from './api/client';
import { LoginView } from './features/auth/LoginView';
import { ChatPanel, PROBLEM_PREFIX } from './features/chat/ChatPanel';
import { HistoryView } from './features/history/HistoryView';
import { ActionsView } from './features/home/ActionsView';
import { Desk } from './features/home/Desk';
import { ProfileView } from './features/home/ProfileView';
import { Skills } from './features/home/Skills';
import { ArticleView } from './features/knowledge/ArticleView';
import { KnowledgeView } from './features/knowledge/KnowledgeView';
import { ProgressPanel } from './features/progress/ProgressPanel';
import { StepDetails } from './features/progress/StepDetails';
import { NewRequest } from './features/requests/NewRequest';
import { ScheduleView } from './features/schedule/ScheduleView';
import { useReminders } from './features/schedule/useReminders';
import { Sidebar } from './features/shell/Sidebar';
import {
  allowed,
  defaultView,
  matchPath,
  navigate,
  pathForView,
  sectionOf,
  viewsFor,
  type RouteMatch,
  type ViewId,
} from './features/shell/views';
import { TaskDetails } from './features/tasks/TaskDetails';
import { TasksView } from './features/tasks/TasksView';
import { InternCard } from './features/team/InternCard';
import { TeamPanel } from './features/team/TeamPanel';
import { guessRole, useChatStore } from './store/chat';

const SUBTITLES: Partial<Record<ViewId, string>> = {
  desk: 'Начните с карточки или спросите Бадди своими словами',
  plan: 'Чек-лист адаптации: шаги с заявками закрываются сами после согласования',
  tasks: 'Задачи адаптации команды: Бадди подбирает следующую по вашим навыкам',
  schedule: 'Встречи команды и ваши созвоны; о каждой Бадди напомнит в чате',
  actions: 'Черновики, которые ждут отправки, и решения руководителя',
  knowledge: 'Инструкции команды, на которые опирается Бадди: видны только разделы для вашей роли',
  history: 'Ваши прошлые диалоги с Бадди',
  skills: 'Готовые вопросы: выберите — и Бадди ответит в чате',
  team: 'Кто просит помощи, что ждёт решения и чего нет в базе знаний',
  chat: 'Проверьте, как Бадди отвечает стажёрам',
  profile: 'Кто вы в команде и как идёт адаптация',
};

const DETAIL_TITLES: Partial<Record<RouteMatch['kind'], [string, string]>> = {
  step: ['Шаг адаптации', 'Что сделать и как шаг закрывается'],
  task: ['Задача', 'Что сделать, когда задача считается выполненной и где прочитать'],
  article: ['Статья базы знаний', 'Те же разделы, на которые ссылается Бадди'],
  'new-request': ['Новая заявка', 'Заполните форму — заявка уйдёт только после вашего подтверждения'],
  intern: ['Карточка стажёра', 'Прогресс, сигналы, заявки и задачи стажёра вашей команды'],
};

// Адрес страницы как внешний источник: useSyncExternalStore перечитывает его после подписки,
// поэтому переход, сделанный дочерним компонентом до подписки App, не теряется.
const subscribeLocation = (cb: () => void) => {
  window.addEventListener('popstate', cb);
  return () => window.removeEventListener('popstate', cb);
};
const locationSnapshot = () => window.location.pathname + window.location.search;

export function App() {
  const user = useChatStore((s) => s.user);
  const signedIn = useChatStore((s) => s.signedIn);
  const signOut = useChatStore((s) => s.signOut);
  const prefill = useChatStore((s) => s.prefill);
  const ask = useChatStore((s) => s.ask);

  // заголовок X-Demo-User должен смениться до любых запросов
  setDemoUser(user);
  useEffect(() => setDemoUser(user), [user]);

  const href = useSyncExternalStore(subscribeLocation, locationSnapshot);
  const q = href.indexOf('?');
  const loc = q < 0 ? { pathname: href, search: '' } : { pathname: href.slice(0, q), search: href.slice(q) };

  if (!signedIn) {
    return <SignedOut pathname={loc.pathname} search={loc.search} />;
  }
  return (
    <Workspace
      user={user}
      loc={loc}
      prefill={prefill}
      ask={ask}
      signOut={signOut}
    />
  );
}

/** Без входа доступен только экран входа; после входа — возврат на открытый адрес (?next=). */
function SignedOut({ pathname, search }: { pathname: string; search: string }) {
  useEffect(() => {
    if (pathname !== '/login') {
      const next = pathname === '/' ? '' : `?next=${encodeURIComponent(pathname + search)}`;
      navigate(`/login${next}`, { replace: true });
    }
  }, [pathname, search]);
  const next = new URLSearchParams(search).get('next');
  return <LoginView next={next && next.startsWith('/') && !next.startsWith('//') ? next : null} />;
}

function Workspace({
  user,
  loc,
  prefill,
  ask,
  signOut,
}: {
  user: string;
  loc: { pathname: string; search: string };
  prefill: (t: string) => void;
  ask: (t: string) => void;
  signOut: () => void;
}) {
  const me = useQuery({ queryKey: ['me', user], queryFn: api.me });
  // учётка на момент входа: переключение в «Войти как» уводит на стартовый раздел новой роли
  const previousUser = useRef(user);
  useReminders();
  const unreadReminders = useChatStore((s) => s.unreadReminders);
  const readReminders = useChatStore((s) => s.readReminders);
  // пока /me грузится после смены учётки — роль по логину, чтобы не мелькал чужой экран
  const role = me.data?.role ?? guessRole(user);
  const isTeamView = role === 'buddy' || role === 'lead';

  // учётка неизвестна серверу (401) — на экран входа
  useEffect(() => {
    if (me.error instanceof ApiError && me.error.status === 401) signOut();
  }, [me.error, signOut]);

  // растёт после «Сбросить демо»: ключ для перемонтирования чата, стола и кабинета
  const [epoch, setEpoch] = useState(0);
  const views = viewsFor(isTeamView);
  const route = matchPath(loc.pathname, isTeamView, loc.search);
  const active = sectionOf(route, isTeamView);
  const forbidden = !allowed(route, isTeamView);
  const chatView: ViewId = isTeamView ? 'chat' : 'desk';
  const isView = route.kind === 'view' && !forbidden;
  const chatVisible = isView && (active === chatView || active === 'chat');

  const go = (next: ViewId, replace = false) => navigate(pathForView(next), { replace });

  // /login после входа и смена учётки — на стартовый раздел роли
  useEffect(() => {
    if (route.kind === 'login') go(defaultView(isTeamView), true);
  }, [route.kind, isTeamView]);
  useEffect(() => {
    if (previousUser.current === user) return;
    previousUser.current = user;
    go(defaultView(isTeamView), true);
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
  }, [loc.pathname, user, epoch]);

  const openChat = (text = '') => {
    go(chatView);
    prefill(text);
  };
  const report = () => openChat(PROBLEM_PREFIX);
  const askBuddy = (q: string) => {
    go(chatView);
    ask(q);
  };
  // «Сбросить демо»: стартовый раздел и заново смонтированные чат и кабинет (без набранного текста и черновиков ответов)
  const afterDemoReset = () => {
    go(defaultView(isTeamView), true);
    setEpoch((e) => e + 1);
  };

  const home = () => go(defaultView(isTeamView));
  let special: ReactNode = null;
  if (route.kind === 'not-found') {
    special = (
      <section className="sheet sheet--state" role="status">
        <p>Такой страницы нет. Проверьте адрес или вернитесь к рабочему разделу.</p>
        <button className="btn btn--link" onClick={home}>
          На главную
        </button>
      </section>
    );
  } else if (forbidden) {
    special = (
      <section className="sheet sheet--state" role="alert">
        <p>Этот раздел недоступен для вашей роли.</p>
        <button className="btn btn--link" onClick={home}>
          Вернуться в свой кабинет
        </button>
      </section>
    );
  } else if (route.kind === 'step') {
    special = <StepDetails stepId={route.stepId} onBack={() => go('plan')} />;
  } else if (route.kind === 'task') {
    special = <TaskDetails key={route.taskId} taskId={route.taskId} onAsk={askBuddy} />;
  } else if (route.kind === 'article') {
    special = <ArticleView key={route.docId} docId={route.docId} />;
  } else if (route.kind === 'new-request') {
    special = <NewRequest key={route.type} type={route.type} />;
  } else if (route.kind === 'intern') {
    special = <InternCard key={route.internId} internId={route.internId} canDecide={role === 'lead'} />;
  } else if (route.kind === 'view') {
    switch (route.view) {
      case 'profile':
        special = (
          <ProfileView user={me.data} isLoading={me.isLoading} isError={me.isError} onRetry={() => void me.refetch()} onSignOut={signOut} />
        );
        break;
      case 'tasks':
        special = <TasksView />;
        break;
      case 'knowledge':
        special = <KnowledgeView />;
        break;
      case 'history':
        special = <HistoryView />;
        break;
    }
  }

  const detail = DETAIL_TITLES[route.kind];
  // у стажёра чат живёт на рабочем столе, но /chat открывает его во весь экран
  const current = views.find((v) => v.id === active) ?? (active === 'chat' ? { title: 'Чат с Бадди' } : views[0]);
  const [title, subtitle] =
    route.kind === 'not-found'
      ? ['Страница не найдена', '']
      : forbidden
        ? ['Доступ запрещён', '']
        : detail ?? [current.title, SUBTITLES[active] ?? ''];

  return (
    <div className="app">
      <div className="shell">
        <Sidebar
          views={views}
          view={active}
          staff={isTeamView}
          me={me.data}
          onNavigate={(v) => go(v)}
          badges={chatVisible || unreadReminders === 0 ? {} : { [chatView]: unreadReminders }}
          onWrite={() => openChat()}
          onReport={report}
          onDemoReset={afterDemoReset}
          onSignOut={signOut}
        />

        <main className="workspace" ref={workspaceRef}>
          {/* Навигация — только боковое меню: вкладки над контентом дублировали его один в один */}
          <header className="workspace__head">
            <div className="workspace__titles">
              <h1 className="workspace__title">{title}</h1>
              {subtitle && <p className="workspace__subtitle">{subtitle}</p>}
            </div>
          </header>

          <div
            className={`workspace__body${chatVisible ? ' workspace__body--chat' : ''}${
              isView && active === 'desk' && !isTeamView ? ' workspace__body--desk' : ''
            }`}
          >
            {special}

            {/* чат и стол всегда смонтированы: переписка и поток ответов не теряются при переходах по разделам */}
            <div className="persistent" hidden={special !== null}>
              {!isTeamView && (
                <div className="desk-slot" hidden={active !== 'desk'}>
                  <Desk
                    key={`desk-${epoch}`}
                    onOpenPlan={() => go('plan')}
                    onOpenActions={() => go('actions')}
                    onOpenTasks={() => go('tasks')}
                    onReport={report}
                  />
                </div>
              )}

              <div className={`chat-card${isTeamView || active === 'chat' ? ' chat-card--full' : ''}`} hidden={!chatVisible}>
                <ChatPanel key={`chat-${user}-${epoch}`} staff={isTeamView} />
              </div>

              <div hidden={active !== 'schedule'}>{active === 'schedule' && <ScheduleView key={`schedule-${user}`} />}</div>

              {isTeamView ? (
                <div hidden={active !== 'team'}>
                  <TeamPanel key={`team-${user}-${epoch}`} role={role} />
                </div>
              ) : (
                <>
                  <div hidden={active !== 'plan'}>
                    <ProgressPanel withActions={false} onOpenStep={(id) => navigate(`/plan/${encodeURIComponent(id)}`)} />
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
          </div>

          <footer className="footer">
            Прототип на синтетических данных. Инструкции тестовые и не являются регламентами Авито.
          </footer>
        </main>
      </div>
    </div>
  );
}
