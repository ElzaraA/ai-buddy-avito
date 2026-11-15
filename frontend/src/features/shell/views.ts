import type { ComponentType } from 'react';
import { IconCalendar, IconChat, IconInbox, IconPlan, IconSpark, IconTeam } from './icons';

/** Разделы рабочего пространства. Стажёру — рабочий стол с чатом, наставнику и руководителю — кабинет. */
export type ViewId = 'desk' | 'plan' | 'schedule' | 'actions' | 'skills' | 'team' | 'chat';

export interface ViewDef {
  id: ViewId;
  /**
   * Название раздела: одно и то же в боковом меню и в заголовке рабочей области,
   * чтобы пункт меню и открывшийся экран назывались одинаково.
   */
  title: string;
  icon: ComponentType<{ className?: string }>;
  group: 'main' | 'help';
}

export const INTERN_VIEWS: ViewDef[] = [
  { id: 'desk', title: 'Рабочий стол', icon: IconChat, group: 'main' },
  { id: 'plan', title: 'Мой план', icon: IconPlan, group: 'main' },
  { id: 'schedule', title: 'Расписание', icon: IconCalendar, group: 'main' },
  { id: 'actions', title: 'Мои заявки', icon: IconInbox, group: 'main' },
  { id: 'skills', title: 'Что умеет Бадди', icon: IconSpark, group: 'help' },
];

export const STAFF_VIEWS: ViewDef[] = [
  { id: 'team', title: 'Кабинет команды', icon: IconTeam, group: 'main' },
  { id: 'chat', title: 'Чат с Бадди', icon: IconChat, group: 'main' },
  { id: 'schedule', title: 'Расписание', icon: IconCalendar, group: 'main' },
];

export const viewsFor = (staff: boolean) => (staff ? STAFF_VIEWS : INTERN_VIEWS);
export const defaultView = (staff: boolean): ViewId => (staff ? 'team' : 'desk');
