import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import { useGuard } from '../../components/useGuard';
import { useChatStore } from '../../store/chat';
import { PROBLEM_PREFIX } from '../chat/ChatPanel';
import { MOODS, MoodFace, moodLabel } from './mood';

export const PULSE_TITLE = 'Как вы сегодня?';
export const NEXT_STEP_QUESTION = 'Что мне делать дальше?';
export const LOW_MOOD_TEXT =
  'Спасибо, что честно. Так бывает — давайте разгрузим день: расскажите наставнику, что мешает, или выберем с Бадди один следующий шаг.';
export const HIGH_MOOD_TEXT = 'Здорово! Хороший день, чтобы закрыть следующий шаг плана.';
export const MID_MOOD_TEXT = 'Принято. Если день изменится — отметьте заново.';
/** Подтверждение после «Рассказать наставнику»: что наставник уже видит и что сделать дальше. */
export const TOLD_TEXT =
  'Наставник уже видит вашу отметку в «Пульсе команды». Допишите в чате, что мешает, — Бадди передаст сообщение.';

/**
 * «Пульс» на рабочем столе: одна отметка самочувствия в день (можно поменять).
 * При «Тяжело» / «Так себе» — тёплый ответ и два выхода: наставник или Бадди.
 * Если сервер «Пульс» не поддерживает — блок просто не показывается.
 */
export function PulseStrip() {
  const user = useChatStore((s) => s.user);
  const prefill = useChatStore((s) => s.prefill);
  const ask = useChatStore((s) => s.ask);
  const qc = useQueryClient();
  const pulse = useQuery({ queryKey: ['pulse', user], queryFn: api.myPulse });
  const submit = useMutation({
    mutationFn: (mood: number) => api.submitPulse(mood),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['pulse', user] });
      void qc.invalidateQueries({ queryKey: ['achievements', user] });
    },
  });

  const guard = useGuard();
  const [told, setTold] = useState(false);

  if (pulse.isError || pulse.isLoading || !pulse.data) return null;

  const saved = pulse.data.entries.find((e) => e.date === pulse.data.today)?.mood ?? null;
  // пока ответ сервера в пути — сразу показываем выбор
  const today = submit.isPending ? (submit.variables ?? saved) : (submit.data?.mood ?? saved);

  return (
    <section className="pulse" aria-labelledby="pulse-title">
      <div className="pulse__row">
        <h2 id="pulse-title" className="pulse__title">
          {PULSE_TITLE}
        </h2>
        <div className="pulse__moods" role="group" aria-label={PULSE_TITLE}>
          {MOODS.map((m) => (
            <button
              key={m.value}
              type="button"
              className={`mood-btn${today === m.value ? ' mood-btn--on' : ''}`}
              aria-pressed={today === m.value}
              disabled={submit.isPending}
              onClick={() => {
                setTold(false);
                guard((done) => submit.mutate(m.value, { onSettled: done }));
              }}
            >
              <MoodFace mood={m.value} size={22} />
              <span>{m.label}</span>
            </button>
          ))}
        </div>
      </div>
      <div className="pulse__feedback" role="status">
        {submit.isError && <span className="error">Не удалось сохранить отметку, попробуйте ещё раз.</span>}
        {!submit.isError && today !== null && (
          <>
            <p className="pulse__msg">
              <span className="pulse__today">Сегодня: {moodLabel(today)}.</span>{' '}
              {today <= 2 ? (told ? TOLD_TEXT : LOW_MOOD_TEXT) : today >= 4 ? HIGH_MOOD_TEXT : MID_MOOD_TEXT}
            </p>
            {today <= 2 && (
              <div className="pulse__actions">
                <button
                  type="button"
                  className="btn btn--small btn--primary"
                  onClick={() => {
                    setTold(true);
                    prefill(PROBLEM_PREFIX);
                  }}
                >
                  Рассказать наставнику
                </button>
                <button type="button" className="btn btn--small btn--ghost" onClick={() => ask(NEXT_STEP_QUESTION)}>
                  {NEXT_STEP_QUESTION}
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </section>
  );
}
