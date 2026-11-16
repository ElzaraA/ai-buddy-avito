import { useQuery } from '@tanstack/react-query';
import { ApiError, api } from '../../api/client';
import { STAGE_LABELS } from '../../components/format';
import { useChatStore } from '../../store/chat';
import { Link } from '../shell/Link';

/** Убираем markdown-разметку списков и выделения: статья читается как текст, без сырого синтаксиса. */
function paragraphs(text: string) {
  return text
    .split(/\n{2,}/)
    .map((p) => p.trim())
    .filter(Boolean);
}

function Paragraph({ text }: { text: string }) {
  const lines = text.split('\n');
  const isList = lines.every((l) => /^\s*(?:[-*]|\d+\.)\s+/.test(l));
  const clean = (s: string) => s.replace(/^\s*(?:[-*]|\d+\.)\s+/, '').replace(/\*\*(.+?)\*\*/g, '$1');
  if (isList) {
    const ordered = /^\s*\d+\./.test(lines[0]);
    const items = lines.map((l, i) => <li key={i}>{clean(l)}</li>);
    return ordered ? <ol>{items}</ol> : <ul>{items}</ul>;
  }
  return <p>{lines.map(clean).join(' ')}</p>;
}

/** Статья базы знаний: только разделы, которые видит роль пользователя в его команде. */
export function ArticleView({ docId }: { docId: string }) {
  const user = useChatStore((s) => s.user);
  const doc = useQuery({ queryKey: ['article', user, docId], queryFn: () => api.article(docId), retry: false });

  if (doc.isLoading) return <section className="sheet sheet--state" role="status">Загружаю статью…</section>;
  if (doc.isError || !doc.data) {
    const missing = doc.error instanceof ApiError && doc.error.status === 404;
    return (
      <section className="sheet sheet--state">
        <p className={missing ? undefined : 'error'} role={missing ? 'status' : 'alert'}>
          {missing ? 'Статья не найдена или недоступна для вашей роли.' : 'Не удалось загрузить статью.'}
        </p>
        {missing ? (
          <Link to="/knowledge" className="btn btn--link">
            Ко всем статьям
          </Link>
        ) : (
          <button type="button" className="btn btn--link" onClick={() => void doc.refetch()}>
            Повторить
          </button>
        )}
      </section>
    );
  }
  const d = doc.data;
  return (
    <article className="sheet sheet--wide article" aria-labelledby="article-title">
      <Link to="/knowledge" className="btn btn--link back-link">
        ← Все статьи
      </Link>
      <header className="sheet__head">
        <h2 id="article-title">{d.title}</h2>
        <span className="sheet__meta">{d.id}</span>
      </header>
      {d.sections.map((s, i) => (
        <section key={`${s.title}-${i}`} className="article__section" aria-label={s.title}>
          {s.title !== d.title && <h3>{s.title}</h3>}
          {s.stage && <span className="chip chip--light">{STAGE_LABELS[s.stage] ?? s.stage}</span>}
          {paragraphs(s.text).map((p, j) => (
            <Paragraph key={j} text={p} />
          ))}
        </section>
      ))}
      <p className="block__note">{d.disclaimer}</p>
    </article>
  );
}
