import type { Task } from '../../api/schemas';

export const TASK_STATUS: Record<Task['status'], string> = {
  todo: 'Не начата',
  in_progress: 'В работе',
  done: 'Сделано',
  skipped: 'Отложена',
};

export const difficultyLabel = (d: number) => `Сложность ${d} из 5`;
export const estimateLabel = (h: number) => (h > 0 ? `≈ ${h} ч` : '');
