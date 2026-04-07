/**
 * Electron preload script — context bridge between main and renderer.
 * Exposes a safe, typed API on window.electron.
 * Runs in an isolated context; nodeIntegration is disabled.
 */

import { contextBridge, ipcRenderer } from 'electron';

// Inject the backend base URL so the React API client can use absolute URLs
// when the app is loaded from file:// (packaged mode).
const backendUrl = (process.env.BACKEND_URL as string | undefined) ?? 'http://localhost:8080';
(globalThis as unknown as Record<string, unknown>).__ELECTRON_API_BASE__ = `${backendUrl}/api/v1`;
(globalThis as unknown as Record<string, unknown>).__ELECTRON__ = true;

export type AppInfo = {
  version: string;
  platform: string;
  isPackaged: boolean;
  backendUrl: string;
};

// Expose on window.electron
contextBridge.exposeInMainWorld('electron', {
  /** Minimise the current window */
  minimize: () => ipcRenderer.send('window:minimize'),

  /** Toggle maximise/restore for the current window */
  maximize: () => ipcRenderer.send('window:maximize'),

  /** Close the current window */
  close: () => ipcRenderer.send('window:close'),

  /** Open a new independent window (optional URL / title) */
  openWindow: (opts?: { url?: string; title?: string }) =>
    ipcRenderer.send('window:openNew', opts ?? {}),

  /** Lock all windows — clears auth and redirects to login */
  lockScreen: () => ipcRenderer.send('screen:lock'),

  /** Query app metadata (version, platform, backendUrl) */
  getAppInfo: (): Promise<AppInfo> => ipcRenderer.invoke('app:info'),

  /** Open a URL in the default OS browser */
  openExternal: (url: string) => ipcRenderer.send('shell:openExternal', url),

  /** Show a native message dialog */
  showMessage: (opts: {
    type?: 'info' | 'warning' | 'error' | 'question';
    title?: string;
    message: string;
    detail?: string;
  }) => ipcRenderer.send('dialog:showMessage', opts),

  /** Platform string for conditional UI */
  platform: process.platform,
});

// TypeScript declaration for consumers (window.electron)
declare global {
  interface Window {
    electron: {
      minimize(): void;
      maximize(): void;
      close(): void;
      openWindow(opts?: { url?: string; title?: string }): void;
      lockScreen(): void;
      getAppInfo(): Promise<AppInfo>;
      openExternal(url: string): void;
      showMessage(opts: {
        type?: 'info' | 'warning' | 'error' | 'question';
        title?: string;
        message: string;
        detail?: string;
      }): void;
      platform: string;
    };
    __ELECTRON__: boolean;
    __ELECTRON_API_BASE__: string;
  }
}
