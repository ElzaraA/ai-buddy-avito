import '@testing-library/jest-dom/vitest';
import { useChatStore } from '../store/chat';

// Тесты экранов работают от имени уже вошедшего пользователя; экран входа проверяется отдельно.
beforeEach(() => {
  useChatStore.setState({ signedIn: true });
});
