import type { AnchorHTMLAttributes, MouseEvent } from 'react';
import { navigate } from './views';

/**
 * Ссылка внутри приложения: настоящий href (открыть в новой вкладке, скопировать адрес),
 * а обычный клик — переход без перезагрузки страницы.
 */
export function Link({ to, onClick, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement> & { to: string }) {
  const handle = (e: MouseEvent<HTMLAnchorElement>) => {
    onClick?.(e);
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    navigate(to);
  };
  return <a href={to} onClick={handle} {...rest} />;
}
