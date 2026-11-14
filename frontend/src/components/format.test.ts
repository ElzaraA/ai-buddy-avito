import { fieldLabel, fieldValue, formatDate, hasFieldLabel, stageLabel, statusLabel } from './format';

describe('format', () => {
  it('formatDate переводит ISO в дд.мм.гггг', () => {
    expect(formatDate('2026-11-20')).toBe('20.11.2026');
    expect(formatDate('завтра')).toBe('завтра');
    expect(formatDate(undefined)).toBe('');
  });

  it('stageLabel и fieldLabel дают подписи и не теряют неизвестные ключи', () => {
    expect(stageLabel('week1')).toBe('Первая неделя');
    expect(stageLabel('x')).toBe('x');
    expect(fieldLabel('request_id')).toBe('Номер заявки');
    expect(fieldLabel('foo')).toBe('foo');
  });

  it('statusLabel и fieldValue', () => {
    expect(statusLabel('pending')).toBe('ждёт согласования');
    expect(statusLabel('weird')).toBe('weird');
    expect(statusLabel('cancelled')).toBe('отменена');
    expect(fieldValue('status', 'pending')).toBe('ждёт согласования');
    expect(fieldValue('created_at', '2026-11-10T19:17:28+03:00')).toBe('10.11.2026 19:17');
    expect(fieldValue('created_at', 'нет даты')).toBe('нет даты');
    expect(fieldValue('resource', 'jira')).toBe('jira');
    expect(fieldValue('dateFrom', '2026-11-20')).toBe('20.11.2026');
    expect(fieldValue('date_to', '2026-11-21')).toBe('21.11.2026');
  });

  it('hasFieldLabel: policy подписан, служебные флаги — нет', () => {
    expect(fieldLabel('policy')).toBe('Порядок');
    expect(hasFieldLabel('policy')).toBe(true);
    expect(hasFieldLabel('suggest_notify_buddy')).toBe(false);
    expect(hasFieldLabel('toString')).toBe(false);
  });
});
