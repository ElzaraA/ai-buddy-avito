import { useQuery } from '@tanstack/react-query';
import { ApiError, api } from '../../api/client';
import { formatDate, resourceLabel, statusLabel } from '../../components/format';
import { useChatStore } from '../../store/chat';
import { Link } from '../shell/Link';
import { RecommendedList, sourceNote } from '../tasks/TasksView';
import { TASK_STATUS } from '../tasks/taskText';

/** Карточка стажёра своей команды: прогресс, «Пульс», обращения, заявки и задачи. */
export function InternCard({ internId, canDecide }: { internId: string; canDecide: boolean }) {
  const user = useChatStore((s) => s.user);
  const card = useQuery({ queryKey: ['team-intern', user, internId], queryFn: () => api.teamIntern(internId), retry: false });

  if (card.isLoading) return <section className="sheet sheet--state" role="status">Загружаю карточку стажёра…</section>;
  if (card.isError || !card.data) {
    const missing = card.error instanceof ApiError && card.error.status === 404;
    return (
      <section className="sheet sheet--state">
        <p className={missing ? undefined : 'error'} role={missing ? 'status' : 'alert'}>
          {missing ? 'Такого стажёра нет в вашей команде.' : 'Не удалось загрузить карточку.'}
        </p>
        <Link to="/team" className="btn btn--link">
          В кабинет команды
        </Link>
      </section>
    );
  }
  const d = card.data;
  const pending = d.actions.filter((a) => a.status === 'pending');
  const openSignals = d.signals.filter((s) => !s.done);

  return (
    <article className="sheet sheet--wide intern-card" aria-labelledby="intern-title">
      <Link to="/team" className="btn btn--link back-link">
        ← Кабинет команды
      </Link>
      <header className="sheet__head">
        <h2 id="intern-title">{d.user.name}</h2>
        <span className="sheet__meta">команда {d.user.team_id}</span>
      </header>

      <div className="summary">
        <div className="summary__item">
          <span className="summary__value">{Math.round(d.progress.percent_done)}%</span>
          <span className="summary__label">план адаптации</span>
        </div>
        <div className="summary__item">
          <span className="summary__value">{d.pulse.today ?? '—'}</span>
          <span className="summary__label">«Пульс» сегодня{d.pulse.avg7 != null ? `, в среднем ${d.pulse.avg7.toFixed(1)}` : ''}</span>
        </div>
        <div className="summary__item">
          <span className="summary__value">{openSignals.length}</span>
          <span className="summary__label">открытых обращений</span>
        </div>
        {d.tasks && (
          <div className="summary__item">
            <span className="summary__value">{d.tasks.summary.done}</span>
            <span className="summary__label">задач сделано, в работе {d.tasks.summary.in_progress}</span>
          </div>
        )}
      </div>
      {d.pulse.risk && (
        <p className="note note--risk" role="status">
          Нужно внимание: {d.pulse.risk_reason}
        </p>
      )}

      <section className="block" aria-labelledby="ic-plan">
        <h3 className="block__title" id="ic-plan">
          План адаптации
        </h3>
        <p className="muted">{d.progress.next_step ? `Следующий шаг: ${d.progress.next_step}` : 'Все шаги выполнены'}</p>
        <ul className="steps">
          {d.progress.items.map((it) => (
            <li key={it.id} className={`step${it.done ? ' step--done' : ''}`}>
              <span className="step__title">{it.title}</span>
              <span className="step__hint">{it.done ? 'сделано' : it.status === 'waiting' ? 'ждёт заявки' : 'впереди'}</span>
            </li>
          ))}
        </ul>
      </section>

      <section className="block" aria-labelledby="ic-signals">
        <h3 className="block__title" id="ic-signals">
          Обращения к наставнику
        </h3>
        {d.signals.length === 0 ? (
          <p className="muted">Стажёр пока не обращался.</p>
        ) : (
          <ul className="signals">
            {d.signals.map((s) => (
              <li key={s.id} className={`signal${s.done ? ' signal--done' : ''}`}>
                <span className="signal__text">{s.text}</span>
                <span className="muted">
                  {s.at ? formatDate(s.at) : ''} · {s.done ? 'разобрано' : 'ждёт ответа'}
                </span>
              </li>
            ))}
          </ul>
        )}
        {openSignals.length > 0 && (
          <Link to="/team" className="btn btn--link">
            Ответить в кабинете команды
          </Link>
        )}
      </section>

      <section className="block" aria-labelledby="ic-actions">
        <h3 className="block__title" id="ic-actions">
          Заявки
        </h3>
        {d.actions.length === 0 ? (
          <p className="muted">Отправленных заявок нет.</p>
        ) : (
          <ul className="ledger">
            {d.actions.map((a) => (
              <li key={a.id} className="ledger__row">
                <span className="ledger__id">{a.id}</span>
                <span className="ledger__title">
                  {a.type === 'access'
                    ? `Доступ: ${resourceLabel(a.resource)}`
                    : `Отгул ${formatDate(a.date_from)}${a.date_to && a.date_to !== a.date_from ? ` – ${formatDate(a.date_to)}` : ''}`}
                </span>
                <span className={`badge badge--${a.status}`}>{statusLabel(a.status)}</span>
              </li>
            ))}
          </ul>
        )}
        {pending.length > 0 && (
          <p className="muted">
            {canDecide ? (
              <Link to="/team" className="btn btn--link">
                Согласовать в кабинете команды ({pending.length})
              </Link>
            ) : (
              `Ждут решения руководителя: ${pending.length}`
            )}
          </p>
        )}
      </section>

      {d.tasks && (
        <section className="block" aria-labelledby="ic-tasks">
          <h3 className="block__title" id="ic-tasks">
            Задачи
          </h3>
          {d.tasks.active.length === 0 ? (
            <p className="muted">Стажёр ещё не брал задачи.</p>
          ) : (
            <ul className="task-list">
              {d.tasks.active.map((t) => (
                <li key={t.id} className={`task-row task-row--${t.status}`}>
                  <span className="task-row__title">{t.title}</span>
                  <span className={`badge badge--task-${t.status}`}>{TASK_STATUS[t.status]}</span>
                </li>
              ))}
            </ul>
          )}
          {d.tasks.recommendations.items.length > 0 && (
            <>
              <h4 className="block__subtitle">Что Бадди советует стажёру дальше</h4>
              <RecommendedList items={d.tasks.recommendations.items} compact linkable={false} />
              <p className="block__note">{sourceNote(d.tasks.recommendations.source)}</p>
            </>
          )}
        </section>
      )}
    </article>
  );
}
