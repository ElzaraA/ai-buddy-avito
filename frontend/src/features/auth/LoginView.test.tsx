import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithQuery } from '../../test/utils';
import { useChatStore } from '../../store/chat';
import { LoginView } from './LoginView';

describe('LoginView', () => {
  it('демо-вход: честная пометка, выбор учётки и переход на запрошенный адрес', async () => {
    useChatStore.setState({ signedIn: false });
    window.history.replaceState(null, '', '/login');
    renderWithQuery(<LoginView next="/tasks" />);
    expect(screen.getByText(/вход без пароля, все учётки и данные синтетические/)).toBeInTheDocument();
    expect(screen.getByText('После входа откроется /tasks')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: /Наставник Backend/ }));
    expect(useChatStore.getState()).toMatchObject({ user: 'mentor.backend', signedIn: true });
    expect(sessionStorage.getItem('ai-buddy.user')).toBe('mentor.backend');
    expect(window.location.pathname).toBe('/tasks');

    useChatStore.getState().signOut();
    expect(useChatStore.getState().signedIn).toBe(false);
    expect(sessionStorage.getItem('ai-buddy.user')).toBeNull();
    useChatStore.getState().setUser('intern.backend');
  });
});
