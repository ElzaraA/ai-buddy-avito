/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Базовый URL API. Пусто = тот же origin (dev-прокси Vite или nginx). */
  readonly VITE_API_URL?: string;
}
