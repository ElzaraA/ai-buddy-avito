import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import { stageLabel } from '../../components/format';
import { useChatStore } from '../../store/chat';
import { closesByApproval } from './ProgressPanel';

export function StepDetails({ stepId, onBack }: { stepId: string; onBack: () => void }) {
  const user = useChatStore((state) => state.user);
  const queryClient = useQueryClient();
  const progress = useQuery({ queryKey: ['progress', user], queryFn: api.progress });
  const complete = useMutation({
    mutationFn: () => api.completeStep(stepId),
    onSuccess: (next) => queryClient.setQueryData(['progress', user], next),
  });

  if (progress.isLoading) return <section className="sheet sheet--state" role="status">Загружаю шаг…</section>;
  if (progress.isError || !progress.data) {
    return (
      <section className="sheet sheet--state">
        <p className="error" role="alert">Не удалось загрузить шаг адаптации.</p>
        <button className="btn btn--link" onClick={() => void progress.refetch()}>Повторить</button>
      </section>
    );
  }

  const item = progress.data.items.find((candidate) => candidate.id === stepId);
  if (!item) {
    return (
      <section className="sheet sheet--state" role="status">
        <h2>Шаг не найден</h2>
        <button className="btn btn--link" onClick={onBack}>Вернуться к плану</button>
      </section>
    );
  }

  return (
    <section className="sheet sheet--wide" aria-label={`Шаг: ${item.title}`}>
      <button className="btn btn--link" onClick={onBack}>← Мой план</button>
      <p className="eyebrow">{stageLabel(item.stage)}</p>
      <h2>{item.title}</h2>
      <p className="muted">Статус: {item.done ? 'Выполнен' : item.status === 'waiting' ? 'Ждёт согласования' : 'Не выполнен'}</p>
      {item.owner && <p>Ответственный: {item.owner}</p>}
      {closesByApproval(item) && !item.done ? (
        <p className="muted">Этот шаг завершится после решения по заявке. Отметить его вручную нельзя.</p>
      ) : !item.done && (
        <button className="btn btn--primary" onClick={() => complete.mutate()} disabled={complete.isPending}>
          {complete.isPending ? 'Сохраняю…' : 'Отметить выполненным'}
        </button>
      )}
      {complete.isError && <p className="error" role="alert">{(complete.error as Error).message}</p>}
    </section>
  );
}