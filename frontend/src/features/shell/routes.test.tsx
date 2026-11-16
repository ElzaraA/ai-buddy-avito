import { fireEvent, render, screen } from '@testing-library/react';
import { Link } from './Link';
import { allowed, matchPath, navigate, sectionOf } from './views';

describe('маршруты', () => {
  it('разбирает разделы, детальные экраны и неизвестные адреса', () => {
    expect(matchPath('/', false)).toEqual({ kind: 'view', view: 'desk' });
    expect(matchPath('/', true)).toEqual({ kind: 'view', view: 'team' });
    expect(matchPath('/tasks/')).toEqual({ kind: 'view', view: 'tasks' });
    expect(matchPath('/tasks/be-02')).toEqual({ kind: 'task', taskId: 'be-02' });
    expect(matchPath('/knowledge/tools.md')).toEqual({ kind: 'article', docId: 'tools.md' });
    expect(matchPath('/team/interns/intern-1')).toEqual({ kind: 'intern', internId: 'intern-1' });
    expect(matchPath('/plan/%E2%9C%93')).toEqual({ kind: 'step', stepId: '✓' });
    expect(matchPath('/plan/%E0%A4%A')).toEqual({ kind: 'not-found' });
    expect(matchPath('/actions/new', false, '?type=absence')).toEqual({ kind: 'new-request', type: 'absence' });
    expect(matchPath('/actions/new', false, '?type=hack')).toEqual({ kind: 'new-request', type: 'access' });
    expect(matchPath('/login')).toEqual({ kind: 'login' });
    expect(matchPath('/nope/deeper')).toEqual({ kind: 'not-found' });
  });

  it('права по ролям: детальный экран наследует раздел', () => {
    const task = matchPath('/tasks/be-02');
    const intern = matchPath('/team/interns/intern-1');
    expect(sectionOf(task, false)).toBe('tasks');
    expect(allowed(task, false)).toBe(true);
    expect(allowed(task, true)).toBe(false); // наставник не берёт задачи
    expect(allowed(intern, false)).toBe(false); // стажёр не видит карточки коллег
    expect(allowed(intern, true)).toBe(true);
    expect(allowed(matchPath('/knowledge/a.md'), true)).toBe(true);
    expect(allowed(matchPath('/chat'), false)).toBe(true);
    expect(allowed(matchPath('/actions/new'), true)).toBe(false);
    expect(allowed(matchPath('/missing'), true)).toBe(true); // 404 показывается всем
    expect(sectionOf({ kind: 'login' }, true)).toBe('team');
  });

  it('navigate меняет историю и сообщает приложению; Link — обычная ссылка с переходом без перезагрузки', () => {
    const seen: string[] = [];
    const on = () => seen.push(window.location.pathname + window.location.search);
    window.addEventListener('popstate', on);
    navigate('/tasks');
    navigate('/tasks'); // тот же адрес — без лишней записи в истории
    navigate('/knowledge?q=1', { replace: true });
    render(
      <Link to="/history" onClick={() => seen.push('click')}>
        История
      </Link>,
    );
    const link = screen.getByRole('link', { name: 'История' });
    expect(link).toHaveAttribute('href', '/history');
    fireEvent.click(link);
    fireEvent.click(link, { ctrlKey: true }); // открыть в новой вкладке — браузер сам
    window.removeEventListener('popstate', on);
    expect(seen).toEqual(['/tasks', '/tasks', '/knowledge?q=1', 'click', '/history', 'click']);
  });
});
