import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { User } from '../../api/schemas';
import { percentDone } from '../progress/ProgressPanel';
import { useChatStore } from '../../store/chat';

const ROLE_LABEL: Record<string, string> = {
  intern: 'Стажёр',
  buddy: 'Наставник',
  lead: 'Руководитель',
};

export function ProfileView({
  user,
  isLoading,
  isError,
  onRetry,
}: {
  user?: User;
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
}) {
  const login = useChatStore((state) => state.user);
  const isIntern = user?.role === 'intern';
  const progress = useQuery({
    queryKey: ['progress', login],
    queryFn: api.progress,
    enabled: isIntern,
  });
  const pulse = useQuery({
    queryKey: ['my-pulse', login],
    queryFn: api.myPulse,
    enabled: isIntern,
  });

  if (isLoading) {
    return <section className="sheet sheet--state" role="status">Загружаю профиль…</section>;
  }
  if (isError || !user) {
    return (
      <section className="sheet sheet--state">
        <p className="error" role="alert">Не удалось загрузить текущего пользователя.</p>
        <button className="btn btn--link" onClick={onRetry}>Повторить</button>
      </section>
    );
  }
  const latestPulse = pulse.data?.entries[pulse.data.entries.length - 1];

  return (
    <section className="sheet sheet--wide" aria-label="Профиль пользователя">
      <header className="sheet__head">
        <h2>{user.name}</h2>
        <span className="sheet__meta">{ROLE_LABEL[user.role] ?? user.role}</span>
      </header>
      <dl className="profile-facts">
        <div><dt>Команда</dt><dd>{user.team_id}</dd></div>
        <div><dt>Учётная запись</dt><dd>{login}</dd></div>
      </dl>
      {isIntern && (
        <div className="profile-summary">
          <section aria-label="Прогресс адаптации">
            <h3>Адаптация</h3>
            {progress.isLoading ? <p className="muted">Загружаю прогресс…</p> : progress.isError ? (
              <p className="error" role="alert">Не удалось загрузить прогресс.</p>
            ) : (
              <p>{progress.data?.percent_done ?? percentDone(progress.data?.items ?? [])}% выполнено</p>
            )}
          </section>
          <section aria-label="Последняя отметка пульса">
            <h3>Пульс</h3>
            {pulse.isLoading ? <p className="muted">Загружаю отметку…</p> : pulse.isError ? (
              <p className="error" role="alert">Не удалось загрузить отметку пульса.</p>
            ) : (
              <p>{latestPulse
                ? `${latestPulse.date === pulse.data?.today ? 'Сегодня' : latestPulse.date}: ${latestPulse.mood} из 5`
                : 'Сегодня отметки нет'}</p>
            )}
          </section>
        </div>
      )}
    </section>
  );
}