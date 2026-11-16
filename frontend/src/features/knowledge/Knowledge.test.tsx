import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { mockFetch, renderWithQuery } from '../../test/utils';
import { ArticleView } from './ArticleView';
import { KnowledgeView } from './KnowledgeView';

afterEach(() => vi.unstubAllGlobals());

describe('KnowledgeView', () => {
  it('список статей, поиск с фрагментами и сброс', async () => {
    const fetch = mockFetch([
      {
        match: '/api/v1/knowledge',
        body: { documents: [{ id: 'access.md', title: 'Как получить доступы', stages: ['week1'], disclaimer: 'тест' }] },
      },
      {
        match: /\/api\/v1\/knowledge\?q=/,
        body: {
          query: 'gitlab',
          hits: [{ doc_id: 'access.md', title: 'Как получить доступы', section: 'GitLab', snippet: 'Оформите заявку…' }],
        },
      },
    ]);
    renderWithQuery(<KnowledgeView />);
    const link = await screen.findByRole('link', { name: 'Как получить доступы' });
    expect(link).toHaveAttribute('href', '/knowledge/access.md');
    expect(screen.getByText(/Первая неделя · access.md/)).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText('Поиск по базе знаний'), 'доступ gitlab');
    await userEvent.click(screen.getByRole('button', { name: 'Найти' }));
    expect(await screen.findByText('Оформите заявку…')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Как получить доступы — GitLab' })).toBeInTheDocument();
    expect(fetch.mock.calls.some(([u]) => String(u).includes('q=%D0%B4%D0%BE%D1%81%D1%82%D1%83%D0%BF%20gitlab'))).toBe(true);

    await userEvent.click(screen.getByRole('button', { name: 'Сбросить' }));
    expect(screen.getByLabelText('Поиск по базе знаний')).toHaveValue('');
    expect(screen.getByRole('link', { name: 'Как получить доступы' })).toBeInTheDocument();
  });

  it('ничего не нашлось — подсказка; сбой списка — «Повторить»', async () => {
    mockFetch([
      { match: '/api/v1/knowledge', status: 500, body: {} },
      { match: /\/api\/v1\/knowledge\?q=/, body: { query: 'парковка', hits: [] } },
    ]);
    renderWithQuery(<KnowledgeView />);
    expect(await screen.findByText(/Не удалось загрузить базу знаний/)).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Поиск по базе знаний'), 'парковка');
    await userEvent.click(screen.getByRole('button', { name: 'Найти' }));
    expect(await screen.findByText(/ничего не нашлось/)).toBeInTheDocument();
  });
});

describe('ArticleView', () => {
  it('разделы со списками без markdown-разметки и дисклеймер', async () => {
    mockFetch([
      {
        match: '/api/v1/knowledge/access.md',
        body: {
          id: 'access.md',
          title: 'Как получить доступы',
          stages: ['week1'],
          disclaimer: 'Тестовая инструкция',
          sections: [
            { title: 'Как получить доступы', stage: 'week1', text: 'Вступление **важно**.' },
            { title: 'GitLab', stage: 'week1', text: 'Что сделать:\n\n1. Напишите Бадди\n2. Подтвердите заявку\n\n- пункт' },
          ],
        },
      },
    ]);
    renderWithQuery(<ArticleView docId="access.md" />);
    expect(await screen.findByRole('heading', { name: 'Как получить доступы', level: 2 })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'GitLab', level: 3 })).toBeInTheDocument();
    expect(screen.getByText('Вступление важно.')).toBeInTheDocument();
    expect(screen.getByText('Подтвердите заявку').tagName).toBe('LI');
    expect(screen.getByText('пункт').closest('ul')).not.toBeNull();
    expect(screen.getByText('Тестовая инструкция')).toBeInTheDocument();
  });

  it('закрытая статья — 404 без подробностей; сбой — «Повторить»', async () => {
    mockFetch([{ match: '/api/v1/knowledge/mentors.md', status: 404, body: { error: { code: 'not_found', message: 'x' } } }]);
    const { unmount } = renderWithQuery(<ArticleView docId="mentors.md" />);
    expect(await screen.findByText('Статья не найдена или недоступна для вашей роли.')).toBeInTheDocument();
    unmount();
    mockFetch([{ match: '/api/v1/knowledge/a.md', status: 500, body: {} }]);
    renderWithQuery(<ArticleView docId="a.md" />);
    expect(await screen.findByRole('button', { name: 'Повторить' })).toBeInTheDocument();
  });
});
