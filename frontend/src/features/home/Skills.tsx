import { useState } from 'react';
import { SUGGESTION_GROUPS } from '../chat/ChatPanel';

const TONES = ['green', 'blue', 'pink'] as const;
const ALL = 'Все';

/** «Что умеет Бадди»: готовые вопросы по задачам стажёра. Клик — вопрос уходит в чат. */
export function Skills({ onAsk }: { onAsk: (question: string) => void }) {
  const [filter, setFilter] = useState(ALL);
  const groups = SUGGESTION_GROUPS.map((g, i) => ({ ...g, tone: TONES[i % TONES.length] }));
  const rows = groups
    .filter((g) => filter === ALL || g.title === filter)
    .flatMap((g) => g.items.map((q) => ({ q, group: g })));

  return (
    <section className="panel skills" aria-label="Что умеет Бадди">
      <p className="panel__lead">
        Бадди знает правила вашей команды, оформляет заявки и подсказывает следующий шаг. Выберите вопрос — он
        отправится в чат.
      </p>
      <div className="chips" role="group" aria-label="Фильтр подсказок">
        {[ALL, ...groups.map((g) => g.title)].map((t) => (
          <button
            key={t}
            type="button"
            className={`filter${filter === t ? ' filter--active' : ''}`}
            aria-pressed={filter === t}
            onClick={() => setFilter(t)}
          >
            {t}
          </button>
        ))}
      </div>
      <ul className="rows">
        {rows.map(({ q, group }) => (
          <li key={q}>
            <button type="button" className="list-row" onClick={() => onAsk(q)}>
              <span className={`chip chip--${group.tone}`}>{group.title}</span>
              <span className="list-row__body">
                <span className="list-row__title">{q}</span>
                <span className="list-row__text">{group.hint}</span>
              </span>
              <span className="list-row__go" aria-hidden>
                →
              </span>
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
