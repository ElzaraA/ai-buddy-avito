import { screen } from '@testing-library/react';
import { AssistantMessage, mentionsBuddy } from './AssistantMessage';
import { renderWithQuery } from '../test/utils';

describe('AssistantMessage', () => {
  it('answer: текст и уникальные источники', () => {
    renderWithQuery(
      <AssistantMessage
        response={{
          type: 'answer',
          text: 'Код-ревью делает наставник',
          sources: [
            { id: 'a#1', source: 'tools.md' },
            { id: 'a#2', source: 'tools.md' },
            { id: 'b#1' },
          ],
        }}
      />,
    );
    expect(screen.getByText('Код-ревью делает наставник')).toBeInTheDocument();
    expect(screen.getAllByText('tools.md')).toHaveLength(1);
    expect(screen.getByText('b#1')).toBeInTheDocument();
  });

  it('refusal: предлагает обратиться к наставнику', () => {
    renderWithQuery(<AssistantMessage response={{ type: 'refusal', buddy_name: 'Иван Петров' }} />);
    expect(screen.getByText('Не могу помочь с этим вопросом.')).toBeInTheDocument();
    expect(screen.getByText('Наставник: Иван Петров')).toBeInTheDocument();
  });

  it('refusal: имя наставника уже в тексте — вторую строку не дублирует', () => {
    renderWithQuery(
      <AssistantMessage
        response={{
          type: 'refusal',
          text: 'Не нашёл ответа в доступных источниках. Лучше написать наставнику — Ивану Петрову.',
          buddy_name: 'Иван Петров',
        }}
      />,
    );
    expect(screen.getByText(/Ивану Петрову/)).toBeInTheDocument();
    expect(screen.queryByText(/^Наставник:/)).not.toBeInTheDocument();
  });

  it('mentionsBuddy: узнаёт фамилию в любом падеже', () => {
    expect(mentionsBuddy('Обратитесь к Анне Козловой', 'Анна Козлова')).toBe(true);
    expect(mentionsBuddy('Обратитесь к HR BP', 'Анна Козлова')).toBe(false);
    expect(mentionsBuddy(undefined, 'Иван Петров')).toBe(false);
    expect(mentionsBuddy('текст', '')).toBe(false);
  });

  it('action: карточка результата со скалярными полями, без вложенных списков', () => {
    renderWithQuery(
      <AssistantMessage
        response={{
          type: 'action',
          result: {
            ok: true,
            message: 'Заявка на доступ создана',
            data: { request_id: 'REQ-1', status: 'pending', items: [{ a: 1 }] },
          },
        }}
      />,
    );
    expect(screen.getByText(/Заявка на доступ создана/)).toBeInTheDocument();
    expect(screen.getByText('REQ-1')).toBeInTheDocument();
    expect(screen.getByText('ждёт согласования')).toBeInTheDocument();
    expect(screen.queryByText('items')).not.toBeInTheDocument();
  });

  it('action absence_get_policy: порядок с русской подписью, без сырых ключей', () => {
    renderWithQuery(
      <AssistantMessage
        response={{
          type: 'action',
          result: {
            ok: true,
            message: 'Порядок оформления отгула',
            data: {
              policy: 'Согласовать с руководителем, затем создать заявку в системе отсутствий.',
              manager_name: 'Мария Сидорова',
              buddy_name: 'Иван Петров',
              suggest_notify_buddy: true,
              internal_flag: 'x',
            },
          },
        }}
      />,
    );
    expect(screen.getByText('Порядок')).toBeInTheDocument();
    expect(screen.getByText(/Согласовать с руководителем/)).toBeInTheDocument();
    expect(screen.getByText('Руководитель')).toBeInTheDocument();
    expect(screen.queryByText('policy')).not.toBeInTheDocument();
    expect(screen.queryByText('internal_flag')).not.toBeInTheDocument();
    expect(screen.queryByText('suggest_notify_buddy')).not.toBeInTheDocument();
  });

  it('action: неуспех и пустой результат', () => {
    renderWithQuery(<AssistantMessage response={{ type: 'action', result: { ok: false, message: '' } }} />);
    expect(screen.getByText(/Действие выполнено/)).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveClass('card--error');
  });

  it('draft: показывает черновик', () => {
    renderWithQuery(<AssistantMessage response={{ type: 'draft', action: 'absence', args: { date_from: '2026-11-20' } }} />);
    expect(screen.getByText(/Черновик заявки на отгул/)).toBeInTheDocument();
  });

  it('draft без args', () => {
    renderWithQuery(<AssistantMessage response={{ type: 'draft', action: 'absence' }} />);
    expect(screen.getByText(/Напишите дату отгула/)).toBeInTheDocument();
  });

  it('degraded: резервная модель — честная пометка под ответом', () => {
    renderWithQuery(<AssistantMessage response={{ type: 'answer', text: 'Через GitLab', degraded: true }} />);
    expect(screen.getByText('Через GitLab')).toBeInTheDocument();
    expect(screen.getByRole('note')).toHaveTextContent(/упрощённом режиме/);
  });

  it('обычный ответ — без пометки', () => {
    renderWithQuery(<AssistantMessage response={{ type: 'answer', text: 'Через GitLab' }} />);
    expect(screen.queryByRole('note')).not.toBeInTheDocument();
  });
});
