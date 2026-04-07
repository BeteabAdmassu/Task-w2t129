/**
 * System tray management for the MedOps desktop console.
 * Provides: open, backup, lock, new window, configurable reminders, and quit.
 *
 * Daily backup reminder persistence:
 *   The date of the last fired reminder is stored in a plain JSON file at
 *   `<userData>/tray-state.json` (via electron's `app.getPath('userData')`).
 *   On startup the file is read; the reminder fires at most once per calendar day.
 */

import { Tray, Menu, Notification, app } from 'electron';
import { readFileSync, writeFileSync, existsSync } from 'fs';
import { join } from 'path';

type TrayOptions = {
  icon: Electron.NativeImage;
  onOpen: () => void;
  onLock: () => void;
  onNewWindow: () => void;
  onBackup: () => void;
  onQuit: () => void;
};

let tray: Tray | null = null;
let reminderInterval: ReturnType<typeof setInterval> | null = null;
let dailyReminderTimer: ReturnType<typeof setTimeout> | null = null;
let reminderMinutes = 0; // 0 = off

// ─── Persistent state ─────────────────────────────────────────────────────────

function statePath(): string {
  return join(app.getPath('userData'), 'tray-state.json');
}

interface TrayState {
  lastBackupReminderDate: string | null; // ISO date string YYYY-MM-DD
}

function loadState(): TrayState {
  try {
    if (existsSync(statePath())) {
      return JSON.parse(readFileSync(statePath(), 'utf8')) as TrayState;
    }
  } catch {
    // Corrupt file — start fresh
  }
  return { lastBackupReminderDate: null };
}

function saveState(state: TrayState): void {
  try {
    writeFileSync(statePath(), JSON.stringify(state), 'utf8');
  } catch {
    // Non-fatal — reminder will just fire again next launch
  }
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10); // YYYY-MM-DD
}

// ─── Public API ───────────────────────────────────────────────────────────────

/** Set up the system tray icon and context menu. */
export function setupTray(opts: TrayOptions): void {
  if (tray) return; // already set up

  tray = new Tray(opts.icon);
  tray.setToolTip('MedOps Offline Operations Console');
  tray.on('double-click', opts.onOpen);

  rebuildMenu(opts);

  // Schedule once-per-day backup reminder.
  scheduleDailyBackupReminder(opts);
}

/** Tear down the tray on app quit. */
export function cleanupTray(): void {
  clearReminder();
  if (dailyReminderTimer !== null) {
    clearTimeout(dailyReminderTimer);
    dailyReminderTimer = null;
  }
  tray?.destroy();
  tray = null;
}

// ─── Menu builder ─────────────────────────────────────────────────────────────

function rebuildMenu(opts: TrayOptions): void {
  if (!tray) return;

  const reminderLabel =
    reminderMinutes > 0
      ? `Reminders: every ${reminderMinutes} min ✓`
      : 'Reminders: off';

  const menu = Menu.buildFromTemplate([
    {
      label: 'Open MedOps Console',
      click: opts.onOpen,
    },
    { type: 'separator' },
    {
      label: 'Backup Now',
      click: () => {
        opts.onBackup();
        fireNotification('MedOps Backup', 'Backup started. You will be notified when it completes.', opts.onOpen);
      },
    },
    { type: 'separator' },
    {
      label: 'Lock Screen',
      accelerator: 'CmdOrCtrl+L',
      click: () => {
        opts.onLock();
        tray?.setToolTip('MedOps Console — Locked');
      },
    },
    { type: 'separator' },
    {
      label: reminderLabel,
      submenu: Menu.buildFromTemplate(
        [0, 15, 30, 60].map((minutes) => ({
          label: minutes === 0 ? 'Off' : `Every ${minutes} minutes`,
          type: 'radio' as const,
          checked: reminderMinutes === minutes,
          click: () => {
            reminderMinutes = minutes;
            setReminder(minutes, opts);
            rebuildMenu(opts);
          },
        })),
      ),
    },
    { type: 'separator' },
    {
      label: 'New Window',
      accelerator: 'CmdOrCtrl+N',
      click: opts.onNewWindow,
    },
    { type: 'separator' },
    {
      label: `Version ${app.getVersion()}`,
      enabled: false,
    },
    {
      label: 'Quit MedOps',
      accelerator: 'CmdOrCtrl+Q',
      click: opts.onQuit,
    },
  ]);

  tray.setContextMenu(menu);
}

// ─── Periodic reminder system (configurable interval) ─────────────────────────

function clearReminder(): void {
  if (reminderInterval !== null) {
    clearInterval(reminderInterval);
    reminderInterval = null;
  }
}

function setReminder(minutes: number, opts: TrayOptions): void {
  clearReminder();
  if (minutes === 0) return;

  const ms = minutes * 60 * 1000;
  reminderInterval = setInterval(() => {
    fireNotification(
      'MedOps Reminder',
      'Time to check outstanding work orders and low-stock alerts.',
      opts.onOpen,
    );
  }, ms);
}

// ─── Daily backup reminder ─────────────────────────────────────────────────────
// Fires at most once per calendar day. State persists to disk so restarting the
// app does not re-fire the reminder if it already fired today.

function scheduleDailyBackupReminder(opts: TrayOptions): void {
  const state = loadState();
  const today = todayISO();

  if (state.lastBackupReminderDate !== today) {
    // Has not fired today — schedule it for 30 seconds after startup so the
    // app is fully loaded, then reschedule for subsequent days.
    dailyReminderTimer = setTimeout(() => {
      fireDailyBackupReminder(opts);
    }, 30_000);
  } else {
    // Already fired today — schedule for midnight tonight.
    scheduleNextMidnight(opts);
  }
}

function fireDailyBackupReminder(opts: TrayOptions): void {
  const state = loadState();
  state.lastBackupReminderDate = todayISO();
  saveState(state);

  fireNotification(
    'MedOps Daily Backup Reminder',
    "Don't forget your daily backup. Click to open the console.",
    opts.onOpen,
  );

  // Reschedule for next calendar day midnight.
  scheduleNextMidnight(opts);
}

function scheduleNextMidnight(opts: TrayOptions): void {
  const now = new Date();
  const tomorrow = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
  const msUntilMidnight = tomorrow.getTime() - now.getTime();

  dailyReminderTimer = setTimeout(() => {
    fireDailyBackupReminder(opts);
  }, msUntilMidnight);
}

// ─── Notification helper ───────────────────────────────────────────────────────

function fireNotification(title: string, body: string, onClick: () => void): void {
  if (!Notification.isSupported()) return;

  const n = new Notification({ title, body, timeoutType: 'default' });
  n.on('click', onClick);
  n.show();
}
