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
  Notification,
  safeStorage,
} from 'electron';
import { join } from 'path';
import { existsSync, readFileSync, writeFileSync, mkdirSync, unlinkSync } from 'fs';
import { randomBytes } from 'crypto';
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

/**
 * Resolves the backend binary to run.
 *
 * Priority order:
 *  1. DATA_DIR/active/backend/medops-server[.exe]  — installed by an offline update package
 *  2. process.resourcesPath/backend/medops-server[.exe]  — bundled at packaging time
 *
 * In dev mode (IS_DEV) the backend is assumed to be running externally (Docker / go run)
 * and null is returned so startBackend waits on the health endpoint instead.
 */
function getBackendBinary(): string | null {
  if (IS_DEV) return null; // dev mode: backend runs externally (Docker / go run)

  const binaryName = process.platform === 'win32' ? 'medops-server.exe' : 'medops-server';

  // 1. Check for an active override installed by an offline update.
  const userDataDir = app.getPath('userData');
  const dataDir = join(userDataDir, 'data');
  const activeOverride = join(dataDir, 'active', 'backend', binaryName);
  if (existsSync(activeOverride)) {
    console.log('[main] Using active binary override:', activeOverride);
    return activeOverride;
  }

  // 2. Fall back to the binary bundled at packaging time.
  const candidate = join(process.resourcesPath, 'backend', binaryName);
  return existsSync(candidate) ? candidate : null;
}

/**
 * Resolves the frontend assets path to load in the renderer window.
 *
 * Priority order:
 *  1. DATA_DIR/active/frontend/index.html  — installed by an offline update package
 *  2. __dirname/../dist/index.html         — bundled at packaging time
 *
 * In dev mode the Vite dev server URL is used instead.
 */
function getFrontendPath(): string {
  const VITE_DEV_URL = process.env.VITE_DEV_URL ?? 'http://localhost:3000';
  if (IS_DEV) return VITE_DEV_URL;

  const userDataDir = app.getPath('userData');
  const dataDir = join(userDataDir, 'data');
  const activeFrontend = join(dataDir, 'active', 'frontend', 'index.html');
  if (existsSync(activeFrontend)) {
    console.log('[main] Using active frontend override:', activeFrontend);
    return activeFrontend;
  }

  return join(__dirname, '..', 'dist', 'index.html');
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

    // Ensure secrets exist before starting backend (generates on first run)
    const secrets = ensureSecrets(app.getPath('userData'));

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
        JWT_SECRET: secrets.jwtSecret,
        ENCRYPT_KEY: secrets.encryptKey,
        HMAC_SIGNING_KEY: secrets.hmacKey,
        TENANT_ID: 'default',
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

// ─── Restart flag watcher ─────────────────────────────────────────────────────

/**
 * watchRestartFlag polls for the sentinel file written by the backend's Rollback handler
 * (DATA_DIR/restart.flag).  When found it:
 *  1. Removes the flag so the watcher does not trigger again.
 *  2. Stops the backend subprocess.
 *  3. Starts a new backend subprocess from the (now-restored) active binary.
 *  4. Reloads all renderer windows so they pick up the restored frontend assets.
 *  5. Shows a system tray / desktop notification.
 *
 * This implements the "offline restart" leg of the version rollback — the database
 * has already been restored by the time the flag is written.
 */
function watchRestartFlag(dataDir: string): void {
  const flagPath = join(dataDir, 'restart.flag');
  let handling = false;

  const check = async (): Promise<void> => {
    if (handling) return;
    if (!existsSync(flagPath)) return;

    handling = true;
    let version = 'previous version';
    try {
      version = readFileSync(flagPath, 'utf8').trim() || version;
      unlinkSync(flagPath);
    } catch {
      // Best-effort flag removal; proceed regardless.
    }

    console.log('[main] Restart flag detected — restarting backend for rollback to', version);

    try {
      await stopBackend();
      await startBackend();

      // Reload all renderer windows so they pick up the restored frontend assets.
      for (const win of windows) {
        if (!win.isDestroyed()) {
          const frontendPath = getFrontendPath();
          if (IS_DEV || frontendPath.startsWith('http')) {
            win.loadURL(frontendPath);
          } else {
            win.loadFile(frontendPath);
          }
        }
      }

      new Notification({
        title: 'MedOps — Rollback Complete',
        body: `System has been restored to ${version}. All windows reloaded.`,
      }).show();
    } catch (err) {
      dialog.showErrorBox(
        'MedOps — Restart Failed',
        `The backend could not restart after rollback.\n\n${(err as Error).message}\n\nPlease restart MedOps manually.`,
      );
    } finally {
      handling = false;
    }
  };

  // Poll every 2 seconds — low overhead, no file-system watch dependency.
  setInterval(() => { void check(); }, 2000);
}

// ─── Secret bootstrap ─────────────────────────────────────────────────────────

interface AppSecrets {
  jwtSecret: string;
  encryptKey: string;
  hmacKey: string;
}

/**
 * ensureSecrets — loads or generates the three backend secrets.
 *
 * Storage strategy (in priority order):
 *  1. Encrypted file via Electron safeStorage (OS DPAPI on Windows).
 *     File: <userData>/.secrets.enc  (binary, not human-readable)
 *  2. Plain-text fallback ONLY in dev mode (IS_DEV === true) where
 *     safeStorage may not be available and security requirements are relaxed.
 *
 * On packaged (non-dev) builds safeStorage is always available on Windows 10+.
 * If it is somehow unavailable the app shows an explicit error and quits — it
 * must NOT silently fall back to plaintext storage in production.
 */
function ensureSecrets(userDataDir: string): AppSecrets {
  mkdirSync(userDataDir, { recursive: true });

  const encPath = join(userDataDir, '.secrets.enc');
  const legacyPath = join(userDataDir, '.secrets.json');

  // --- Try loading from encrypted store ---
  if (safeStorage.isEncryptionAvailable()) {
    if (existsSync(encPath)) {
      try {
        const ciphertext = readFileSync(encPath);
        const plaintext = safeStorage.decryptString(ciphertext);
        const parsed = JSON.parse(plaintext) as Partial<AppSecrets>;
        if (parsed.jwtSecret && parsed.encryptKey && parsed.hmacKey) {
          // Clean up legacy plain file if it exists from an older build
          if (existsSync(legacyPath)) {
            try { unlinkSync(legacyPath); } catch { /* ignore */ }
          }
          return parsed as AppSecrets;
        }
      } catch (e) {
        console.warn('[main] Could not decrypt secrets file — regenerating', e);
      }
    }

    // --- Generate and persist new secrets ---
    const secrets: AppSecrets = {
      jwtSecret: randomBytes(32).toString('hex'),
      encryptKey: randomBytes(32).toString('hex'),
      hmacKey: randomBytes(32).toString('hex'),
    };
    const ciphertext = safeStorage.encryptString(JSON.stringify(secrets));
    writeFileSync(encPath, ciphertext, { mode: 0o600 });
    // Remove any legacy plain file
    if (existsSync(legacyPath)) {
      try { unlinkSync(legacyPath); } catch { /* ignore */ }
    }
    console.log('[main] Generated and securely stored app secrets (safeStorage)');
    return secrets;
  }

  // --- safeStorage not available ---
  if (!IS_DEV) {
    // In a packaged build this should never happen on Windows 10+.
    // Show a user-safe error and quit rather than silently using plaintext.
    dialog.showErrorBox(
      'MedOps — security error',
      'OS-level encryption (safeStorage) is not available on this system.\n\n' +
      'MedOps requires OS-level secret protection to start securely.\n' +
      'Ensure you are running Windows 10 or later and that the user profile is not corrupted.',
    );
    app.quit();
    // Unreachable after quit — satisfies TypeScript return type
    return { jwtSecret: '', encryptKey: '', hmacKey: '' };
  }

  // Dev-mode only: plain JSON fallback so developers can run without packaging
  if (existsSync(legacyPath)) {
    try {
      const raw = JSON.parse(readFileSync(legacyPath, 'utf8')) as Partial<AppSecrets>;
      if (raw.jwtSecret && raw.encryptKey && raw.hmacKey) {
        return raw as AppSecrets;
      }
    } catch { /* regenerate */ }
  }
  const devSecrets: AppSecrets = {
    jwtSecret: randomBytes(32).toString('hex'),
    encryptKey: randomBytes(32).toString('hex'),
    hmacKey: randomBytes(32).toString('hex'),
  };
  writeFileSync(legacyPath, JSON.stringify(devSecrets), { mode: 0o600 });
  console.log('[main] Dev mode: stored secrets in plain JSON (safeStorage not available)');
  return devSecrets;
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
    width: 1920,
    height: 1080,
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

  // Resolve the asset path: active override > bundled > dev server.
  const frontendPath = options.url ?? getFrontendPath();

  if (IS_DEV || frontendPath.startsWith('http')) {
    win.loadURL(frontendPath);
  } else {
    win.loadFile(frontendPath);
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

  /**
   * system:restart-backend — stops the running backend subprocess and starts a new one
   * from the currently active binary (which may have just been swapped by a rollback or
   * update).  All renderer windows are reloaded so they pick up the new frontend assets.
   *
   * Called from the renderer via window.electron.restartBackend() after a successful
   * update or rollback, as an alternative to waiting for the polling-based restart flag.
   */
  ipcMain.handle('system:restart-backend', async () => {
    console.log('[main] system:restart-backend IPC received — restarting...');
    await stopBackend();
    await startBackend();
    for (const win of windows) {
      if (!win.isDestroyed()) {
        const frontendPath = getFrontendPath();
        if (IS_DEV || frontendPath.startsWith('http')) {
          win.loadURL(frontendPath);
        } else {
          win.loadFile(frontendPath);
        }
      }
    }
  });
}

// ─── App lifecycle ────────────────────────────────────────────────────────────

// Enable high-DPI (HiDPI / Retina) awareness on all platforms.
// 'high-dpi-support' tells Chromium to honour the OS DPI scaling factor.
// We intentionally do NOT set 'force-device-scale-factor' — that overrides
// the OS setting and would cause blurry rendering on non-96-DPI displays
// (e.g. 150% scaling on 2K monitors). Let the OS scale factor pass through.
// Must be called before app is ready.
app.commandLine.appendSwitch('high-dpi-support', '1');

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

  // Start polling for the restart flag written by the backend's Rollback handler.
  // DATA_DIR is app.getPath('userData')/data — same path injected into the backend env.
  const dataDir = join(app.getPath('userData'), 'data');
  watchRestartFlag(dataDir);

  setupTray({
    icon: createAppIcon(),
    backendUrl: BACKEND_URL,
    getAuthToken: async () => {
      try {
        const win = mainWindow;
        if (win && !win.isDestroyed()) {
          return (await win.webContents.executeJavaScript(
            `localStorage.getItem('medops_token') ?? ''`,
          )) as string;
        }
      } catch {
        // window not ready
      }
      return '';
    },
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
    onBackup: async () => {
      let token = '';
      try {
        const win = mainWindow;
        if (win && !win.isDestroyed()) {
          token = (await win.webContents.executeJavaScript(
            `localStorage.getItem('medops_token') ?? ''`,
          )) as string;
        }
      } catch {
        // window not ready — proceed without token; backend will return 401
      }
      fetch(`${BACKEND_URL}/api/v1/system/backup`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })
        .then((r) => {
          const msg = r.ok
            ? 'Backup completed successfully.'
            : 'Backup failed — check the console logs.';
          new Notification({ title: 'MedOps Backup', body: msg }).show();
        })
        .catch(() => {
          new Notification({
            title: 'MedOps Backup',
            body: 'Backup request failed — backend may be offline.',
          }).show();
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
