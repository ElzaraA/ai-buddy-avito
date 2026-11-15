/** 2026-11-20 → 20.11.2026; некорректную строку возвращает как есть. */
export function formatDate(iso: string | undefined): string {
  if (!iso) return '';
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso);
  return m ? `${m[3]}.${m[2]}.${m[1]}` : iso;
}

export const STAGE_LABELS: Record<string, string> = {
  week1: 'Первая неделя',
  week2: 'Вторая неделя',
  month1: 'Первый месяц',
};

export function stageLabel(stage: string) {
  return STAGE_LABELS[stage] ?? stage;
}

/** Человекочитаемые подписи для полей результата инструмента. */
const FIELD_LABELS: Record<string, string> = {
  request_id: 'Номер заявки',
  created_at: 'Создана',
  absence_id: 'Номер заявки',
  status: 'Статус',
  owner: 'Ответственный',
  resource: 'Система',
  signal_id: 'Номер обращения',
  your_message: 'Ваше сообщение',
  next_step: 'Следующий шаг',
  how_to: 'Как сделать',
  progress: 'Пройдено',
  manager_name: 'Руководитель',
  buddy_name: 'Наставник',
  policy: 'Порядок',
  what_next: 'Что дальше',
  dateFrom: 'С',
  dateTo: 'По',
  date_from: 'С',
  date_to: 'По',
  // ответ на «какие у меня встречи?»
  next_meeting: 'Ближайшая',
  place: 'Где',
  upcoming: 'Дальше',
  // ответ на «добавь созвон завтра в 15:00»
  duration: 'Длительность',
  reminder: 'Напомню',
};

const FIELD_ORDER = Object.keys(FIELD_LABELS);

/** Порядок полей в карточке — как в FIELD_LABELS (сервер отдаёт ключи по алфавиту). */
export function byFieldOrder(a: string, b: string) {
  return FIELD_ORDER.indexOf(a) - FIELD_ORDER.indexOf(b);
}

export function fieldLabel(key: string) {
  return FIELD_LABELS[key] ?? key;
}

/** Есть ли у поля человекочитаемая подпись: поля без подписи в карточке не показываем. */
export function hasFieldLabel(key: string) {
  return Object.prototype.hasOwnProperty.call(FIELD_LABELS, key);
}

/** Системы приходят от агента в нижнем регистре (gitlab) — показываем их так, как их пишут люди. */
const RESOURCE_LABELS: Record<string, string> = {
  gitlab: 'GitLab',
  github: 'GitHub',
  jira: 'Jira',
  grafana: 'Grafana',
  confluence: 'Confluence',
  git: 'Git',
  vpn: 'VPN',
  slack: 'Slack',
  mattermost: 'Mattermost',
};

export function resourceLabel(resource: string | undefined): string {
  if (!resource) return '';
  return RESOURCE_LABELS[resource.trim().toLowerCase()] ?? resource;
}

const STATUS_LABELS: Record<string, string> = {
  draft: 'черновик',
  pending: 'ждёт согласования',
  approved: 'согласована',
  rejected: 'отклонена',
  cancelled: 'отменена',
  granted: 'выдан',
  waiting: 'ждёт заявку',
};

export function statusLabel(s: string) {
  return STATUS_LABELS[s] ?? s;
}

/** Значение поля результата в человекочитаемом виде. */
export function fieldValue(key: string, value: unknown): string {
  const v = String(value);
  if (key === 'status') return statusLabel(v);
  if (key === 'resource') return resourceLabel(v);
  if (/^date_?(from|to)$/i.test(key)) return formatDate(v);
  if (key === 'created_at') {
    const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(v);
    return m ? `${m[3]}.${m[2]}.${m[1]} ${m[4]}:${m[5]}` : v;
  }
  return v;
}
