import type { ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { statusLabel } from '../../components/format';
import { useChatStore } from '../../store/chat';
import { actionTitle, ACTIONS_REFRESH_MS } from '../progress/MyActions';
import { percentDone } from '../progress/ProgressPanel';
import { Achievements } from '../pulse/Achievements';
import { PulseStrip } from '../pulse/PulseStrip';

interface CardProps {
  tone: 'green' | 'blue' | 'pink';
  tag: string;
  title: string;
  text: string;
  pill: string;
  meta: string;
  label: string;
  onClick: () => void;
  children?: ReactNode;
}

function DeskCard({ tone, tag, title, text, pill, meta, label, onClick, children }: CardProps) {
  return (
    <li className={`desk-card desk-card--${tone}`}>
      <button type="button" className="desk-card__btn" onClick={onClick} aria-label={label}>
        <span className="chip chip--light">{tag}</span>
        <span className="desk-card__title">{title}</span>
        <span className="desk-card__text">{text}</span>
        {children}
        <span className="desk-card__foot">
          <span className="pill">{pill}</span>
          <span className="desk-card__meta">{meta}</span>
        </span>
      </button>
    </li>
  );
}

const plural = (n: number, one: string, few: string, many: string) => {
  const m10 = n % 10;
  const m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return one;
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return few;
  return many;
};

interface Props {
  onOpenPlan: () => void;
  onOpenActions: () => void;
  onReport: () => void;
}

/** Три карточки рабочего стола стажёра: что делать дальше, что с заявками, куда идти, если застрял. */
export function Desk({ onOpenPlan, onOpenActions, onReport }: Props) {
  const user = useChatStore((s) => s.user);
  const progress = useQuery({
    queryKey: ['progress', user],
    queryFn: api.progress,
    refetchInterval: ACTIONS_REFRESH_MS,
  });
  const actions = useQuery({
    queryKey: ['actions', user],
    queryFn: api.listActions,
    refetchInterval: ACTIONS_REFRESH_MS,
  });

  const p = progress.data;
  const pct = p ? Math.round(p.percent_done ?? percentDone(p.items)) : 0;
  const nextTitle = progress.isLoading
    ? 'Загружаю план…'
    : !p
      ? 'План пока недоступен'
      : p.next_step || (p.items.length > 0 ? 'Все шаги выполнены' : 'План пока пуст');

  const list = actions.data ?? [];
  const drafts = list.filter((a) => a.status === 'draft').length;
  const pending = list.filter((a) => a.status === 'pending').length;
  // API отдаёт заявки новыми сверху: последняя — первая в списке
  const last = list[0];
  const actionsTitle = actions.isLoading
    ? 'Загружаю заявки…'
    : actions.isError
      ? 'Заявки пока недоступны'
      : drafts > 0
      ? `${drafts} ${plural(drafts, 'черновик ждёт', 'черновика ждут', 'черновиков ждут')} отправки`
      : pending > 0
        ? `${pending} ${plural(pending, 'заявка ждёт', 'заявки ждут', 'заявок ждут')} согласования`
        : list.length > 0
          ? 'Новых решений нет'
          : 'Заявок пока нет';
  const actionsText = last
    ? `Последняя: ${actionTitle(last)} — ${statusLabel(last.status)}`
    : actions.isError
      ? 'Не удалось связаться с сервером — откройте раздел чуть позже'
      : 'Попросите Бадди оформить отгул или доступ — он заполнит заявку сам';

  return (
    <div className="desk">
      {/* «Пульс» и «Достижения» необязательны: если сервер их не отдаёт, блоки просто не показываются */}
      <PulseStrip />
      {/* на широком экране — колонка слева от чата, чтобы поле ввода было видно без прокрутки */}
      <div className="desk__rail">
        <ul className="desk-cards" aria-label="Коротко о главном">
          <DeskCard
            tone="green"
            tag="Мой план"
            title={nextTitle}
            text={p ? 'Следующий шаг в плане адаптации' : 'Чек-лист первых недель'}
            pill="Открыть план"
            meta={p ? `пройдено ${pct}%` : ''}
            label={`Следующий шаг: ${nextTitle}. Открыть план`}
            onClick={onOpenPlan}
          >
            <span className="desk-card__meter" aria-hidden>
              <span style={{ width: `${pct}%` }} />
            </span>
          </DeskCard>
          <DeskCard
            tone="blue"
            tag="Мои заявки"
            title={actionsTitle}
            text={actionsText}
            pill={drafts > 0 ? 'Проверить' : 'Открыть'}
            meta={actions.data ? `всего ${list.length}` : ''}
            label={`Мои заявки: ${actionsTitle}. Открыть заявки`}
            onClick={onOpenActions}
          />
          <DeskCard
            tone="pink"
            tag="Нужна помощь?"
            title="Застряли или что-то не работает"
            text="Напишите пару слов — Бадди передаст наставнику"
            pill="Сообщить"
            meta="наставнику"
            label="Нужна помощь? Сообщить наставнику"
            onClick={onReport}
          />
        </ul>
        <Achievements />
      </div>
    </div>
  );
}
