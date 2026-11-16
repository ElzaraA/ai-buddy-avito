import { FormEvent, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { STAGE_LABELS } from '../../components/format';
import { useChatStore } from '../../store/chat';
import { Link } from '../shell/Link';

const MAX_QUERY = 300;

/** База знаний: статьи команды и поиск по разделам — те же документы и права, что у ответов Бадди. */
export function KnowledgeView() {
  const user = useChatStore((s) => s.user);
  const [draft, setDraft] = useState('');
  const [query, setQuery] = useState('');
  const docs = useQuery({ queryKey: ['knowledge', user], queryFn: api.knowledge });
  const search = useQuery({
    queryKey: ['knowledge-search', user, query],
    queryFn: () => api.searchKnowledge(query),
    enabled: query.length > 0,
  });

  const submit = (e: FormEvent) => {
    e.preventDefault();
    setQuery(draft.trim());
  };

  return (
    <section className="sheet sheet--wide knowledge" aria-label="База знаний">
      <form className="kb-search" role="search" onSubmit={submit}>
        <label className="field kb-search__field">
          <span>Поиск по базе знаний</span>
          <input
            type="search"
            value={draft}
            maxLength={MAX_QUERY}
            placeholder="Например: доступ в GitLab"
            onChange={(e) => setDraft(e.target.value)}
          />
        </label>
        <button type="submit" className="btn btn--primary" disabled={!draft.trim()}>
          Найти
        </button>
        {query && (
          <button
            type="button"
            className="btn btn--link"
            onClick={() => {
              setDraft('');
              setQuery('');
            }}
          >
            Сбросить
          </button>
        )}
      </form>

      {query ? (
        <section className="block" aria-label="Результаты поиска">
          {search.isLoading && <p className="muted">Ищу…</p>}
          {search.isError && (
            <p className="error" role="alert">
              Поиск не удался.{' '}
              <button type="button" className="btn btn--link" onClick={() => void search.refetch()}>
                Повторить
              </button>
            </p>
          )}
          {search.data && search.data.hits.length === 0 && (
            <p className="muted">
              По запросу «{query}» ничего не нашлось. Спросите Бадди своими словами или наставника — вопрос попадёт в
              список того, что стоит дописать в базу.
            </p>
          )}
          <ul className="kb-hits">
            {search.data?.hits.map((h, i) => (
              <li key={`${h.doc_id}-${i}`} className="kb-hit">
                <Link to={`/knowledge/${encodeURIComponent(h.doc_id)}`} className="kb-hit__title">
                  {h.title}
                  {h.section && h.section !== h.title ? ` — ${h.section}` : ''}
                </Link>
                <p className="kb-hit__snippet">{h.snippet}</p>
              </li>
            ))}
          </ul>
        </section>
      ) : (
        <section className="block" aria-label="Статьи">
          {docs.isLoading && <p className="muted">Загружаю статьи…</p>}
          {docs.isError && (
            <p className="error" role="alert">
              Не удалось загрузить базу знаний.{' '}
              <button type="button" className="btn btn--link" onClick={() => void docs.refetch()}>
                Повторить
              </button>
            </p>
          )}
          {docs.data && docs.data.documents.length === 0 && <p className="muted">Для вашей роли статей пока нет.</p>}
          <ul className="kb-docs">
            {docs.data?.documents.map((d) => (
              <li key={d.id} className="kb-doc">
                <Link to={`/knowledge/${encodeURIComponent(d.id)}`} className="kb-doc__title">
                  {d.title}
                </Link>
                <span className="kb-doc__meta">
                  {d.stages.map((s) => STAGE_LABELS[s] ?? s).join(', ')} · {d.id}
                </span>
              </li>
            ))}
          </ul>
        </section>
      )}
      <p className="block__note">Инструкции тестовые и не являются регламентами Авито.</p>
    </section>
  );
}
