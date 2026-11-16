import type { User } from '../../api/schemas';
import { DEMO_USERS, useChatStore } from '../../store/chat';
import { DemoControls } from './DemoControls';
import { BuddyAvatar, IconArrow, IconFlag } from './icons';
import type { ViewDef, ViewId } from './views';

interface Props {
  views: ViewDef[];
  view: ViewId;
  staff: boolean;
  me?: User;
  onNavigate: (v: ViewId) => void;
  /** Счётчики на пунктах меню (например, непрочитанные напоминания у чата). */
  badges?: Partial<Record<ViewId, number>>;
  /** «Написать Бадди» — открыть чат и поставить фокус в поле ввода */
  onWrite: () => void;
  /** «Сообщить наставнику» — открыть чат с заготовкой сообщения (только стажёр) */
  onReport: () => void;
  /** после «Сбросить демо» — вернуться на стартовый раздел с пустым чатом */
  onDemoReset: () => void;
}

function NavItem({
  v,
  active,
  onClick,
  badge = 0,
}: {
  v: ViewDef;
  active: boolean;
  onClick: () => void;
  badge?: number;
}) {
  const Icon = v.icon;
  return (
    <li>
      <button
        type="button"
        className={`nav__item${active ? ' nav__item--active' : ''}`}
        aria-current={active ? 'page' : undefined}
        onClick={onClick}
      >
        <Icon className="nav__icon" />
        <span>{v.title}</span>
        {badge > 0 && (
          <span className="nav__badge" aria-label={`новых напоминаний: ${badge}`}>
            {badge}
          </span>
        )}
      </button>
    </li>
  );
}

export function Sidebar({ views, view, staff, me, onNavigate, badges = {}, onWrite, onReport, onDemoReset }: Props) {
  const user = useChatStore((s) => s.user);
  const setUser = useChatStore((s) => s.setUser);
  const main = views.filter((v) => v.group === 'main');
  const help = views.filter((v) => v.group === 'help');
  const account = views.filter((v) => v.group === 'account');

  return (
    <aside className="sidebar">
      <div className="brand">
        <span className="brand__mark" aria-hidden>
          ai
        </span>
        <span className="brand__text">
          <span className="brand__name">AI-buddy</span>
          <span className="brand__tagline">помощник стажёра</span>
        </span>
      </div>

      <section className="persona" aria-label="Ассистент Бадди">
        <BuddyAvatar />
        <div className="persona__text">
          <span className="persona__name">Бадди</span>
          <span className="persona__status">
            <span className="persona__dot" aria-hidden />
            онлайн · {staff ? 'помогает вашим стажёрам' : 'помогает стажёрам'}
          </span>
        </div>
        <button type="button" className="pill-btn persona__btn" onClick={onWrite}>
          Написать Бадди <IconArrow className="pill-btn__icon" />
        </button>
      </section>

      <nav className="nav" aria-label="Разделы">
        <div className="nav__group">
          <h2 className="nav__heading">{staff ? 'Команда' : 'Работа'}</h2>
          <ul className="nav__list">
            {main.map((v) => (
              <NavItem key={v.id} v={v} active={view === v.id} badge={badges[v.id]} onClick={() => onNavigate(v.id)} />
            ))}
          </ul>
        </div>
        {help.length > 0 && (
          <div className="nav__group">
            <h2 className="nav__heading">Помощь</h2>
            <ul className="nav__list">
              {help.map((v) => (
                <NavItem key={v.id} v={v} active={view === v.id} onClick={() => onNavigate(v.id)} />
              ))}
              {!staff && (
                <li>
                  <button type="button" className="nav__item" onClick={onReport}>
                    <IconFlag className="nav__icon" />
                    <span>Сообщить наставнику</span>
                  </button>
                </li>
              )}
            </ul>
          </div>
        )}
        {account.length > 0 && (
          <div className="nav__group">
            <h2 className="nav__heading">Аккаунт</h2>
            <ul className="nav__list">
              {account.map((v) => (
                <NavItem key={v.id} v={v} active={view === v.id} onClick={() => onNavigate(v.id)} />
              ))}
            </ul>
          </div>
        )}
      </nav>

      <div className="account">
        {me && (
          <div className="me">
            <span className="me__avatar" aria-hidden>
              {me.name.slice(0, 1)}
            </span>
            <span className="me__text">
              <span className="me__name">{me.name}</span>
              <span className="me__team">команда {me.team_id}</span>
            </span>
          </div>
        )}
        <label className="user-switch">
          <span>Войти как</span>
          {/* aria-label задан явно: на телефоне подпись «Войти как» скрыта, а имя поля должно остаться */}
          <select value={user} onChange={(e) => setUser(e.target.value)} aria-label="Войти как">
            {DEMO_USERS.map((u) => (
              <option key={u.login} value={u.login}>
                {u.label}
              </option>
            ))}
          </select>
        </label>
        <DemoControls onReset={onDemoReset} />
      </div>
    </aside>
  );
}
