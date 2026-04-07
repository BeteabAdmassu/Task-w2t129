/**
 * Electron main process — MedOps Offline Operations Console
 * Handles window lifecycle, backend subprocess, tray, and IPC.
 */

import {
  app,
  BrowserWindow,
  ipcMain,
  dialog,
  shell,
  nativeImage,
  session,
} from 'electron';
import { join } from 'path';
import { existsSync } from 'fs';
import { spawn, ChildProcess } from 'child_process';
import { setupTray, cleanupTray } from './tray';

// embedded-postgres is an optional dependency — only available in packaged Electron builds.
// Dynamic import allows the renderer/web build to compile without it installed.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let EmbeddedPostgresClass: any = null;
(async () => {
  try {
    // @ts-ignore — embedded-postgres is an optional runtime dep; types not installed
    const mod = await import('embedded-postgres');
    EmbeddedPostgresClass = mod.default ?? mod;
  } catch {
    // Optional dependency not installed — will use external database
  }
})();

// ─── Constants ────────────────────────────────────────────────────────────────

const IS_DEV = !app.isPackaged;
const BACKEND_PORT = parseInt(process.env.BACKEND_PORT ?? '8080', 10);
const BACKEND_URL = `http://localhost:${BACKEND_PORT}`;

// ─── State ────────────────────────────────────────────────────────────────────

let mainWindow: BrowserWindow | null = null;
let backendProcess: ChildProcess | null = null;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let embeddedPg: any | null = null;
const windows = new Set<BrowserWindow>();

// ─── Backend subprocess ───────────────────────────────────────────────────────

function getBackendBinary(): string | null {
  if (IS_DEV) return null; // dev mode: backend runs externally (Docker / go run)

  const binaryName = process.platform === 'win32' ? 'medops-server.exe' : 'medops-server';
  const candidate = join(process.resourcesPath, 'backend', binaryName);
  return existsSync(candidate) ? candidate : null;
}

// startEmbeddedDatabase starts the bundled PostgreSQL instance via the
// `embedded-postgres` package (required at packaging time).
// In development mode the package is optional and the dev DATABASE_URL is used instead.
async function startEmbeddedDatabase(): Promise<string> {
  if (!EmbeddedPostgresClass) {
    if (!IS_DEV) {
      // Packaged build — embedded-postgres must be present; abort rather than silently
      // connecting to an unanticipated external database.
      dialog.showErrorBox(
        'MedOps — startup error',
        'Embedded database module not found. The installation may be corrupt. Please reinstall MedOps.',
      );
      app.quit();
      // Return is unreachable after quit but satisfies the type-checker.
      return '';
    }
    // Development: allow override via env so `go run` / Docker backends work alongside Electron.
    console.warn('[main] embedded-postgres not available — dev mode, using DATABASE_URL');
    return process.env.DATABASE_URL ?? 'postgres://medops:medops@localhost:5432/medops';
  }
  embeddedPg = new EmbeddedPostgresClass({
    port: 5433,
    user: 'medops',
    password: 'medops',
    database: 'medops',
    dataDir: join(app.getPath('userData'), 'pgdata'),
  });
  await embeddedPg.initialise();
  await embeddedPg.start();
  console.log('[main] Embedded PostgreSQL started on port 5433');
  return 'postgres://medops:medops@localhost:5433/medops';
}

function startBackend(): Promise<void> {
  return new Promise(async (resolve, reject) => {
    const binary = getBackendBinary();

    if (!binary) {
      // Dev mode or binary not found — assume backend is already running
      console.log('[main] Backend binary not found — assuming external backend at', BACKEND_URL);
      waitForBackend(20, resolve, reject);
      return;
    }

    console.log('[main] Starting backend:', binary);

    const userDataDir = join(app.getPath('userData'), 'data');
    const migrationsPath = join(process.resourcesPath, 'backend', 'migrations');

    let databaseUrl: string;
    try {
      databaseUrl = await startEmbeddedDatabase();
      console.log('[main] Embedded PostgreSQL started at', databaseUrl);
    } catch (err) {
      reject(new Error(`Failed to start embedded database: ${(err as Error).message}`));
      return;
    }

    backendProcess = spawn(binary, [], {
      env: {
        ...process.env,
        PORT: String(BACKEND_PORT),
        MIGRATIONS_PATH: migrationsPath,
        DATA_DIR: userDataDir,
        LOG_LEVEL: 'info',
        DATABASE_URL: databaseUrl,
      },
      stdio: ['ignore', 'pipe', 'pipe'],
    });

    backendProcess.stdout?.on('data', (d: Buffer) =>
      console.log('[backend]', d.toString().trim()),
    );
    backendProcess.stderr?.on('data', (d: Buffer) =>
      console.error('[backend]', d.toString().trim()),
    );
    backendProcess.on('exit', (code) => {
      console.warn('[main] Backend exited with code', code);
      backendProcess = null;
    });

    // Wait for backend to become ready
    waitForBackend(60, resolve, reject);
  });
}

function waitForBackend(maxAttempts: number, resolve: () => void, reject: (e: Error) => void): void {
  let attempts = 0;
  const check = (): void => {
    attempts++;
    fetch(`${BACKEND_URL}/api/v1/health`)
      .then((r) => {
        if (r.ok) {
          console.log('[main] Backend ready after', attempts, 'attempt(s)');
          resolve();
        } else {
          retry();
        }
      })
      .catch(() => retry());
  };
  const retry = (): void => {
    if (attempts >= maxAttempts) {
      reject(new Error(`Backend did not become ready after ${maxAttempts} attempts`));
      return;
    }
    setTimeout(check, 1000);
  };
  check();
}

async function stopBackend(): Promise<void> {
  if (backendProcess) {
    console.log('[main] Stopping backend...');
    backendProcess.kill('SIGTERM');
    backendProcess = null;
  }
  await embeddedPg?.stop();
  embeddedPg = null;
}

// ─── Window factory ───────────────────────────────────────────────────────────

function createAppIcon(): Electron.NativeImage {
  // 32x32 teal square encoded as base64 PNG
  const ICON_B64 =
    'iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAAAXNSR0IArs4c6QAAAARnQU1BAACx' +
    'jwv8YQUAAAAJcEhZcwAADsMAAA7DAcdvqGQAAABGSURBVFhH7c4xAQAgDASh/UdrjuAIXCTZ7AIA' +
    'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA8G0BtAABhiK3KeEAAAAASUVORK5CYII=';
  return nativeImage.createFromBuffer(Buffer.from(ICON_B64, 'base64'));
}

function createWindow(options: { url?: string; title?: string } = {}): BrowserWindow {
  const icon = createAppIcon();
  const win = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 960,
    minHeight: 600,
    title: options.title ?? 'MedOps Offline Operations Console',
    icon,
    backgroundColor: '#f5f7fa',
    webPreferences: {
      preload: join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
    show: false, // shown after 'ready-to-show'
  });

  const url = options.url ?? (IS_DEV ? `${BACKEND_URL}` : join(__dirname, '..', 'dist', 'index.html'));

  if (IS_DEV || url.startsWith('http')) {
    win.loadURL(url);
  } else {
    win.loadFile(url);
  }

  win.once('ready-to-show', () => win.show());

  win.on('closed', () => {
    windows.delete(win);
    if (win === mainWindow) mainWindow = null;
  });

  // Open external links in the OS browser
  win.webContents.setWindowOpenHandler(({ url: targetUrl }) => {
    if (targetUrl.startsWith('http')) shell.openExternal(targetUrl);
    return { action: 'deny' };
  });

  windows.add(win);
  return win;
}

// ─── IPC handlers ─────────────────────────────────────────────────────────────

function registerIPC(): void {
  ipcMain.on('window:minimize', (e) => BrowserWindow.fromWebContents(e.sender)?.minimize());
  ipcMain.on('window:maximize', (e) => {
    const win = BrowserWindow.fromWebContents(e.sender);
    if (!win) return;
    win.isMaximized() ? win.unmaximize() : win.maximize();
  });
  ipcMain.on('window:close', (e) => BrowserWindow.fromWebContents(e.sender)?.close());

  ipcMain.on('window:openNew', (_e, opts: { url?: string; title?: string } = {}) => {
    createWindow(opts);
  });

  ipcMain.on('screen:lock', (e) => {
    // Reload the renderer so it hits the auth guard and redirects to /login
    BrowserWindow.fromWebContents(e.sender)?.webContents.reload();
  });

  ipcMain.handle('app:info', () => ({
    version: app.getVersion(),
    platform: process.platform,
    isPackaged: app.isPackaged,
    backendUrl: BACKEND_URL,
  }));

  ipcMain.on('shell:openExternal', (_e, url: string) => {
    if (typeof url === 'string' && url.startsWith('http')) {
      shell.openExternal(url);
    }
  });

  ipcMain.on('dialog:showMessage', async (_e, opts: Electron.MessageBoxOptions) => {
    if (mainWindow) await dialog.showMessageBox(mainWindow, opts);
  });
}

// ─── App lifecycle ────────────────────────────────────────────────────────────

app.whenReady().then(async () => {
  // Security: block navigation to unexpected origins
  session.defaultSession.webRequest.onHeadersReceived((details, callback) => {
    callback({
      responseHeaders: {
        ...details.responseHeaders,
        'Content-Security-Policy': [
          `default-src 'self' ${BACKEND_URL}; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:`,
        ],
      },
    });
  });

  registerIPC();

  try {
    await startBackend();
  } catch (err) {
    await dialog.showErrorBox(
      'Backend Error',
      `Could not start the MedOps backend service.\n\n${(err as Error).message}\n\nPlease ensure PostgreSQL is running on localhost:5432 and try again.`,
    );
    app.quit();
    return;
  }

  mainWindow = createWindow();

  setupTray({
    icon: createAppIcon(),
    onOpen: () => {
      if (mainWindow) {
        mainWindow.show();
        mainWindow.focus();
      } else {
        mainWindow = createWindow();
      }
    },
    onLock: () => {
      for (const win of windows) {
        win.webContents.executeJavaScript(
          `localStorage.removeItem('medops_token'); window.location.href = '/login';`,
        );
      }
    },
    onNewWindow: () => createWindow(),
    onBackup: () => {
      // Trigger backup via the API and notify the user of success/failure.
      fetch(`${BACKEND_URL}/api/v1/system/backup`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${global.__medopsToken ?? ''}` },
      })
        .then((r) => {
          const msg = r.ok ? 'Backup completed successfully.' : 'Backup failed — check the console logs.';
          new Notification({ title: 'MedOps Backup', body: msg }).show();
        })
        .catch(() => {
          new Notification({ title: 'MedOps Backup', body: 'Backup request failed — backend may be offline.' }).show();
        });
    },
    onQuit: () => app.quit(),
  });

  app.on('activate', () => {
    // macOS: re-create window when dock icon is clicked
    if (BrowserWindow.getAllWindows().length === 0) mainWindow = createWindow();
  });
});

app.on('window-all-closed', () => {
  // On macOS apps stay alive in the menu bar; on other platforms quit.
  if (process.platform !== 'darwin') app.quit();
});

app.on('before-quit', () => {
  cleanupTray();
  void stopBackend();
});

// Prevent multiple instances
const gotLock = app.requestSingleInstanceLock();
if (!gotLock) {
  app.quit();
} else {
  app.on('second-instance', () => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.focus();
    }
  });
}
