import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { useChatStore } from '../../store/chat';
import { ACTIONS_REFRESH_MS, MyActions } from '../progress/MyActions';
import { MySignals } from '../progress/MySignals';

/** Раздел «Мои заявки»: черновики и отправленные заявки; если пусто — подсказка, как создать. */
export function ActionsView({ onAsk }: { onAsk: (question: string) => void }) {
  const user = useChatStore((s) => s.user);
  const { data, isLoading } = useQuery({
    queryKey: ['actions', user],
    queryFn: api.listActions,
    refetchInterval: ACTIONS_REFRESH_MS,
  });
  const empty = !isLoading && (data ?? []).length === 0;

  return (
    <section className="sheet sheet--wide" aria-label="Раздел «Мои заявки»">
      {isLoading && <p className="muted">Загружаю заявки…</p>}
      {empty && (
        <div className="empty-state">
          <p className="empty-state__title">Заявок пока нет</p>
          <p className="muted">
            Бадди сам заполнит заявку на отгул или доступ — вам останется проверить и отправить.
          </p>
          <div className="chips">
            <button type="button" className="filter" onClick={() => onAsk('Хочу взять отгул в пятницу')}>
              Взять отгул
            </button>
            <button type="button" className="filter" onClick={() => onAsk('Запроси доступ в GitLab')}>
              Доступ в GitLab
            </button>
          </div>
        </div>
      )}
      <MyActions />
      <MySignals />
    </section>
  );
}
