import type { ComponentType } from 'react';
import { IconChat, IconInbox, IconPlan, IconSpark, IconTeam } from './icons';

/** Разделы рабочего пространства. Стажёру — рабочий стол с чатом, наставнику и руководителю — кабинет. */
export type ViewId = 'desk' | 'plan' | 'actions' | 'skills' | 'team' | 'chat';

export interface ViewDef {
  id: ViewId;
  /** подпись в боковом меню */
  nav: string;
  /** подпись во вкладках над рабочей областью */
  tab: string;
  /** заголовок рабочей области */
  title: string;
  icon: ComponentType<{ className?: string }>;
  group: 'main' | 'help';
}

export const INTERN_VIEWS: ViewDef[] = [
  { id: 'desk', nav: 'Чат с Бадди', tab: 'Рабочий стол', title: 'Рабочий стол', icon: IconChat, group: 'main' },
  { id: 'plan', nav: 'Мой план', tab: 'Мой план', title: 'Мой план', icon: IconPlan, group: 'main' },
  { id: 'actions', nav: 'Мои заявки', tab: 'Заявки', title: 'Мои заявки', icon: IconInbox, group: 'main' },
  { id: 'skills', nav: 'Что умеет Бадди', tab: 'Подсказки', title: 'Что умеет Бадди', icon: IconSpark, group: 'help' },
];

export const STAFF_VIEWS: ViewDef[] = [
  { id: 'team', nav: 'Кабинет команды', tab: 'Кабинет', title: 'Кабинет команды', icon: IconTeam, group: 'main' },
  { id: 'chat', nav: 'Чат', tab: 'Чат', title: 'Чат с Бадди', icon: IconChat, group: 'main' },
];

export const viewsFor = (staff: boolean) => (staff ? STAFF_VIEWS : INTERN_VIEWS);
export const defaultView = (staff: boolean): ViewId => (staff ? 'team' : 'desk');
