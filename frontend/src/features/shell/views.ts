import type { ComponentType } from 'react';
import {
  IconBook,
  IconCalendar,
  IconChat,
  IconHistory,
  IconInbox,
  IconPlan,
  IconProfile,
  IconSpark,
  IconTasks,
  IconTeam,
} from './icons';

/** Разделы рабочего пространства. Стажёру — рабочий стол с чатом, наставнику и руководителю — кабинет. */
export type ViewId =
  | 'desk'
  | 'plan'
  | 'tasks'
  | 'schedule'
  | 'actions'
  | 'knowledge'
  | 'history'
  | 'skills'
  | 'team'
  | 'chat'
  | 'profile';

export interface ViewDef {
  id: ViewId;
  /**
   * Название раздела: одно и то же в боковом меню и в заголовке рабочей области,
   * чтобы пункт меню и открывшийся экран назывались одинаково.
   */
  title: string;
  icon: ComponentType<{ className?: string }>;
  group: 'main' | 'help' | 'account';
}

export const INTERN_VIEWS: ViewDef[] = [
  { id: 'desk', title: 'Рабочий стол', icon: IconChat, group: 'main' },
  { id: 'plan', title: 'Мой план', icon: IconPlan, group: 'main' },
  { id: 'tasks', title: 'Задачи', icon: IconTasks, group: 'main' },
  { id: 'schedule', title: 'Расписание', icon: IconCalendar, group: 'main' },
  { id: 'actions', title: 'Мои заявки', icon: IconInbox, group: 'main' },
  { id: 'knowledge', title: 'База знаний', icon: IconBook, group: 'help' },
  { id: 'history', title: 'История диалогов', icon: IconHistory, group: 'help' },
  { id: 'skills', title: 'Что умеет Бадди', icon: IconSpark, group: 'help' },
  { id: 'profile', title: 'Мой профиль', icon: IconProfile, group: 'account' },
];

export const STAFF_VIEWS: ViewDef[] = [
  { id: 'team', title: 'Кабинет команды', icon: IconTeam, group: 'main' },
  { id: 'chat', title: 'Чат с Бадди', icon: IconChat, group: 'main' },
  { id: 'schedule', title: 'Расписание', icon: IconCalendar, group: 'main' },
  { id: 'knowledge', title: 'База знаний', icon: IconBook, group: 'help' },
  { id: 'history', title: 'История диалогов', icon: IconHistory, group: 'help' },
  { id: 'profile', title: 'Мой профиль', icon: IconProfile, group: 'account' },
];

export const viewsFor = (staff: boolean) => (staff ? STAFF_VIEWS : INTERN_VIEWS);
export const defaultView = (staff: boolean): ViewId => (staff ? 'team' : 'desk');

const VIEW_PATHS: Record<ViewId, string> = {
  desk: '/desk',
  plan: '/plan',
  tasks: '/tasks',
  schedule: '/schedule',
  actions: '/actions',
  knowledge: '/knowledge',
  history: '/history',
  skills: '/skills',
  team: '/team',
  chat: '/chat',
  profile: '/profile',
};

/** Маршрут приложения. Детальные экраны принадлежат разделу (section) — он подсвечивается в меню. */
export type RouteMatch =
  | { kind: 'view'; view: ViewId }
  | { kind: 'step'; stepId: string }
  | { kind: 'task'; taskId: string }
  | { kind: 'article'; docId: string }
  | { kind: 'new-request'; type: 'access' | 'absence' }
  | { kind: 'intern'; internId: string }
  | { kind: 'login' }
  | { kind: 'not-found' };

export function pathForView(view: ViewId) {
  return VIEW_PATHS[view];
}

const DETAIL: { re: RegExp; make: (id: string) => RouteMatch }[] = [
  { re: /^\/plan\/([^/]+)$/, make: (id) => ({ kind: 'step', stepId: id }) },
  { re: /^\/tasks\/([^/]+)$/, make: (id) => ({ kind: 'task', taskId: id }) },
  { re: /^\/knowledge\/([^/]+)$/, make: (id) => ({ kind: 'article', docId: id }) },
  { re: /^\/team\/interns\/([^/]+)$/, make: (id) => ({ kind: 'intern', internId: id }) },
];

export function matchPath(pathname: string, staff = false, search = ''): RouteMatch {
  const path = pathname.length > 1 ? pathname.replace(/\/+$/, '') : pathname;
  if (path === '/') return { kind: 'view', view: defaultView(staff) };
  if (path === '/login') return { kind: 'login' };
  if (path === '/actions/new') {
    const type = new URLSearchParams(search).get('type') === 'absence' ? 'absence' : 'access';
    return { kind: 'new-request', type };
  }
  const view = (Object.keys(VIEW_PATHS) as ViewId[]).find((id) => VIEW_PATHS[id] === path);
  if (view) return { kind: 'view', view };
  for (const d of DETAIL) {
    const m = path.match(d.re);
    if (!m) continue;
    try {
      return d.make(decodeURIComponent(m[1]));
    } catch {
      return { kind: 'not-found' };
    }
  }
  return { kind: 'not-found' };
}

/** Раздел меню, которому принадлежит маршрут (для подсветки и проверки прав). */
export function sectionOf(route: RouteMatch, staff: boolean): ViewId {
  switch (route.kind) {
    case 'view':
      return route.view;
    case 'step':
      return 'plan';
    case 'task':
      return 'tasks';
    case 'article':
      return 'knowledge';
    case 'new-request':
      return 'actions';
    case 'intern':
      return 'team';
    default:
      return defaultView(staff);
  }
}

/** Разрешён ли маршрут роли: раздел должен быть в её меню (сервер всё равно проверяет права сам). */
export function allowed(route: RouteMatch, staff: boolean): boolean {
  if (route.kind === 'login' || route.kind === 'not-found') return true;
  const section = sectionOf(route, staff);
  return viewsFor(staff).some((v) => v.id === section) || (!staff && route.kind === 'view' && route.view === 'chat');
}

/** Переход внутри приложения без перезагрузки: история браузера + событие для App. */
export function navigate(path: string, opts: { replace?: boolean } = {}) {
  const here = window.location.pathname + window.location.search;
  if (here !== path) {
    if (opts.replace) window.history.replaceState(null, '', path);
    else window.history.pushState(null, '', path);
  }
  window.dispatchEvent(new PopStateEvent('popstate'));
}
