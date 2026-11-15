import { useCallback, useRef } from 'react';

/**
 * Защита от двойного клика по кнопке с запросом. `mutation.isPending` у TanStack Query
 * обновляется не синхронно, и второй клик того же двойного клика успевает уйти на сервер
 * (повторное «Отправить заявку» → 409 и ошибка на экране). Замок — синхронный ref.
 *
 *   const guard = useGuard();
 *   onClick={() => guard((done) => m.mutate(v, { onSettled: done }))}
 */
export function useGuard() {
  const busy = useRef(false);
  return useCallback((run: (done: () => void) => void) => {
    if (busy.current) return;
    busy.current = true;
    run(() => {
      busy.current = false;
    });
  }, []);
}
