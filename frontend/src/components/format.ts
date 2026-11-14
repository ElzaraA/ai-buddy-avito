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
  next_step: 'Следующий шаг',
  manager_name: 'Руководитель',
  buddy_name: 'Наставник',
  policy: 'Порядок',
  dateFrom: 'С',
  dateTo: 'По',
  date_from: 'С',
  date_to: 'По',
};

export function fieldLabel(key: string) {
  return FIELD_LABELS[key] ?? key;
}

/** Есть ли у поля человекочитаемая подпись: поля без подписи в карточке не показываем. */
export function hasFieldLabel(key: string) {
  return Object.prototype.hasOwnProperty.call(FIELD_LABELS, key);
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
  if (/^date_?(from|to)$/i.test(key)) return formatDate(v);
  if (key === 'created_at') {
    const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(v);
    return m ? `${m[3]}.${m[2]}.${m[1]} ${m[4]}:${m[5]}` : v;
  }
  return v;
}
