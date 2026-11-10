import { ReactElement } from 'react';
import { render } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { vi } from 'vitest';

export function renderWithQuery(ui: ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return { client, ...render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>) };
}

type Route = { method?: string; match: string | RegExp; status?: number; body: unknown };

/** Простой мок fetch по таблице маршрутов. Возвращает шпиона для проверок. */
export function mockFetch(routes: Route[]) {
  const spy = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const method = (init?.method ?? 'GET').toUpperCase();
    const route = routes.find(
      (r) =>
        (r.method ?? 'GET').toUpperCase() === method &&
        (typeof r.match === 'string' ? url.endsWith(r.match) : r.match.test(url)),
    );
    if (!route) return new Response(JSON.stringify({ error: { code: 'not_found', message: url } }), { status: 404 });
    return new Response(JSON.stringify(route.body), { status: route.status ?? 200 });
  });
  vi.stubGlobal('fetch', spy);
  return spy;
}

/** Управляемый EventSource для тестов SSE. */
export class FakeEventSource {
  static instances: FakeEventSource[] = [];
  onopen: (() => void) | null = null;
  listeners = new Map<string, ((ev: MessageEvent) => void)[]>();
  closed = false;
  constructor(public url: string) {
    FakeEventSource.instances.push(this);
    queueMicrotask(() => this.onopen?.());
  }
  addEventListener(type: string, cb: (ev: MessageEvent) => void) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), cb]);
  }
  emit(type: string, data?: unknown) {
    const ev = { data: data === undefined ? undefined : JSON.stringify(data) } as MessageEvent;
    this.listeners.get(type)?.forEach((cb) => cb(ev));
  }
  close() {
    this.closed = true;
  }
  static last() {
    return FakeEventSource.instances[FakeEventSource.instances.length - 1];
  }
}
