import { screen } from '@testing-library/react';
import { AssistantMessage } from './AssistantMessage';
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
    expect(screen.getByText(/Иван Петров/)).toBeInTheDocument();
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
});
