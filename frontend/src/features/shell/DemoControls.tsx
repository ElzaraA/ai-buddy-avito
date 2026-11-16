import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, ApiError } from '../../api/client';
import type { DemoInfo } from '../../api/schemas';
import { useChatStore } from '../../store/chat';

export const RESET_CONFIRM_TEXT = 'Сбросить все заявки, сигналы и отметки к исходному состоянию?';

/** Подпись бейджа: честно говорим, что это демо и кто отвечает — правила или LLM. */
export function demoBadgeText(d: DemoInfo) {
  const model = d.llm === 'mock' ? 'модель: правила (mock)' : 'модель: LLM';
  return d.demo_mode ? `Демо-режим · ${model}` : model;
}

export function demoExplanation(d: DemoInfo) {
  const model =
    d.llm === 'mock'
      ? 'Ответы собирает детерминированный движок по правилам и базе знаний, без обращения к LLM: так демо работает стабильно и без ключей.'
      : 'Ответы формирует языковая модель (LLM) по базе знаний команды; если она недоступна, ответ помечается как резервный.';
  const data = d.demo_mode ? ' Данные синтетические, их можно сбросить к исходному состоянию.' : '';
  return model + data;
}

/** 404 — ручки /demo у бэкенда нет (или демо выключено): блок просто не показывается. */
const isMissing = (e: unknown) => e instanceof ApiError && e.status === 404;

/**
 * Подвал бокового меню: бейдж «Демо-режим · модель: …» с пояснением и кнопка «Сбросить демо».
 * onReset — оболочка возвращается на стартовый раздел.
 */
export function DemoControls({ onReset }: { onReset: () => void }) {
  const qc = useQueryClient();
  const demo = useQuery({
    queryKey: ['demo'],
    queryFn: api.demo,
    retry: (count, e) => !isMissing(e) && count < 1,
    staleTime: 60_000,
  });
  const reset = useMutation({
    mutationFn: api.resetDemo,
    onSuccess: () => {
      useChatStore.getState().reset();
      void qc.invalidateQueries();
      onReset();
    },
  });

  const d = demo.data;
  if (!d) return null;

  const onClick = () => {
    if (reset.isPending) return;
    if (!window.confirm(RESET_CONFIRM_TEXT)) return;
    reset.mutate();
  };
  const explanation = demoExplanation(d);

  return (
    <div className="demo">
      <details className="demo__info">
        <summary className="demo__badge" title={explanation}>
          <span className={`demo__dot demo__dot--${d.llm}`} aria-hidden />
          {demoBadgeText(d)}
        </summary>
        <p className="demo__text">{explanation}</p>
      </details>
      {d.demo_mode && (
        <button type="button" className="btn btn--small btn--ghost demo__reset" onClick={onClick} disabled={reset.isPending}>
          {reset.isPending ? 'Сбрасываю…' : 'Сбросить демо'}
        </button>
      )}
      {reset.isError && (
        <div className="error demo__error" role="alert">
          {isMissing(reset.error)
            ? 'Сброс недоступен: демо-режим на сервере выключен'
            : `Не удалось сбросить демо: ${(reset.error as Error).message}`}
        </div>
      )}
    </div>
  );
}
