import { DEMO_USERS, guessRole, useChatStore } from '../../store/chat';
import { navigate } from '../shell/views';

const ROLE_HINT: Record<string, string> = {
  intern: 'Чат с Бадди, план адаптации, задачи, заявки и расписание',
  buddy: 'Кабинет команды: кто просит помощи, «Пульс», пробелы базы знаний',
  lead: 'Всё, что видит наставник, плюс согласование заявок',
};

/**
 * Вход в демо-режиме. В Авито пользователь уже авторизован через корпоративный SSO; в прототипе
 * его имитирует выбор синтетической учётки (заголовок X-Demo-User). Пароля нет — и это сказано прямо.
 */
export function LoginView({ next }: { next: string | null }) {
  const setUser = useChatStore((s) => s.setUser);
  const enter = (login: string) => {
    setUser(login);
    navigate(next ?? '/', { replace: true });
  };
  return (
    <div className="login">
      <main className="login__card" aria-labelledby="login-title">
        <div className="brand">
          <span className="brand__mark" aria-hidden>
            ai
          </span>
          <span className="brand__text">
            <span className="brand__name">AI-buddy</span>
            <span className="brand__tagline">помощник стажёра</span>
          </span>
        </div>
        <h1 id="login-title" className="login__title">
          Войти в демо
        </h1>
        <p className="login__lead">
          Демо-режим: вход без пароля, все учётки и данные синтетические. В Авито здесь был бы корпоративный вход.
        </p>
        <ul className="login__list">
          {DEMO_USERS.map((u) => (
            <li key={u.login}>
              <button type="button" className="login__user" onClick={() => enter(u.login)}>
                <span className="login__name">{u.label}</span>
                <span className="login__hint">{ROLE_HINT[guessRole(u.login)]}</span>
              </button>
            </li>
          ))}
        </ul>
        {next && <p className="muted">После входа откроется {next}</p>}
      </main>
    </div>
  );
}
