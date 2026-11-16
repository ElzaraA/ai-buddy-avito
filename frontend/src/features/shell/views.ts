import type { ComponentType } from 'react';
import { IconCalendar, IconChat, IconInbox, IconPlan, IconProfile, IconSpark, IconTeam } from './icons';

/** Разделы рабочего пространства. Стажёру — рабочий стол с чатом, наставнику и руководителю — кабинет. */
export type ViewId = 'desk' | 'plan' | 'schedule' | 'actions' | 'skills' | 'team' | 'chat' | 'profile';

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
  { id: 'chat', title: 'Чат с Бадди', icon: IconChat, group: 'main' },
  { id: 'plan', title: 'Мой план', icon: IconPlan, group: 'main' },
  { id: 'schedule', title: 'Расписание', icon: IconCalendar, group: 'main' },
  { id: 'actions', title: 'Мои заявки', icon: IconInbox, group: 'main' },
  { id: 'skills', title: 'Что умеет Бадди', icon: IconSpark, group: 'help' },
  { id: 'profile', title: 'Мой профиль', icon: IconProfile, group: 'account' },
];

export const STAFF_VIEWS: ViewDef[] = [
  { id: 'team', title: 'Кабинет команды', icon: IconTeam, group: 'main' },
  { id: 'chat', title: 'Чат с Бадди', icon: IconChat, group: 'main' },
  { id: 'schedule', title: 'Расписание', icon: IconCalendar, group: 'main' },
  { id: 'profile', title: 'Мой профиль', icon: IconProfile, group: 'account' },
];

export const viewsFor = (staff: boolean) => (staff ? STAFF_VIEWS : INTERN_VIEWS);
export const defaultView = (staff: boolean): ViewId => (staff ? 'team' : 'desk');

const VIEW_PATHS: Record<ViewId, string> = {
  desk: '/desk',
  plan: '/plan',
  schedule: '/schedule',
  actions: '/actions',
  skills: '/skills',
  team: '/team',
  chat: '/chat',
  profile: '/profile',
};

export type RouteMatch = { kind: 'view'; view: ViewId } | { kind: 'step'; stepId: string } | { kind: 'not-found' };

export function pathForView(view: ViewId) {
  return VIEW_PATHS[view];
}

export function matchPath(pathname: string, staff = false): RouteMatch {
  if (pathname === '/') return { kind: 'view', view: defaultView(staff) };
  const view = (Object.keys(VIEW_PATHS) as ViewId[]).find((id) => VIEW_PATHS[id] === pathname);
  if (view) return { kind: 'view', view };
  const step = pathname.match(/^\/plan\/([^/]+)$/);
  if (step) {
    try {
      return { kind: 'step', stepId: decodeURIComponent(step[1]) };
    } catch {
      return { kind: 'not-found' };
    }
  }
  return { kind: 'not-found' };
}
