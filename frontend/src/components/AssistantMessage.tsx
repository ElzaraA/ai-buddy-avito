import type { AgentResponse } from '../api/schemas';
import { DraftCard } from './DraftCard';
import { fieldLabel, fieldValue } from './format';

/** Отрисовка ответа агента по типу: answer / action / draft / refusal. */
export function AssistantMessage({ response }: { response: AgentResponse }) {
  if (!response.degraded) return <Body response={response} />;
  // основная модель недоступна — ответила резервная; честно говорим об этом пользователю
  return (
    <div className="degraded">
      <Body response={response} />
      <div className="muted" role="note">
        ⚠️ Помощник работает в упрощённом режиме: основная модель временно недоступна.
      </div>
    </div>
  );
}

function Body({ response }: { response: AgentResponse }) {
  switch (response.type) {
    case 'draft':
      return <DraftCard action={response.action ?? 'absence'} args={response.args ?? {}} text={response.text} />;

    case 'action': {
      const r = response.result;
      const data = r?.data ?? {};
      // чек-лист показываем на панели прогресса, в чате — коротко
      const scalar = Object.entries(data).filter(
        ([, v]) => typeof v === 'string' || typeof v === 'number',
      );
      return (
        <div className={`card ${r?.ok ? 'card--success' : 'card--error'}`} role="status">
          <div className="card__title">{r?.ok ? '✅' : '⚠️'} {r?.message || 'Действие выполнено'}</div>
          {scalar.length > 0 && (
            <dl className="kv">
              {scalar.map(([k, v]) => (
                <div key={k}>
                  <dt>{fieldLabel(k)}</dt>
                  <dd>{fieldValue(k, v)}</dd>
                </div>
              ))}
            </dl>
          )}
          <Sources response={response} />
        </div>
      );
    }

    case 'refusal':
      return (
        <div className="bubble bubble--refusal">
          <p>{response.text || 'Не могу помочь с этим вопросом.'}</p>
          {response.buddy_name && (
            <p className="muted">Лучше спросить наставника: {response.buddy_name}</p>
          )}
        </div>
      );

    default:
      return (
        <div className="bubble bubble--assistant">
          <p>{response.text}</p>
          <Sources response={response} />
        </div>
      );
  }
}

function Sources({ response }: { response: AgentResponse }) {
  const list = response.sources ?? [];
  if (list.length === 0) return null;
  const unique = Array.from(new Set(list.map((s) => s.source ?? s.id)));
  return (
    <div className="sources">
      Источники:{' '}
      {unique.map((s) => (
        <span key={s} className="chip">
          {s}
        </span>
      ))}
    </div>
  );
}
