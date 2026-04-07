/**
 * System tray management for the MedOps desktop console.
 * Provides: open, lock, new window, configurable reminders, and quit.
 */

import { Tray, Menu, Notification, nativeImage, app } from 'electron';

type TrayOptions = {
  icon: Electron.NativeImage;
  onOpen: () => void;
  onLock: () => void;
  onNewWindow: () => void;
  onQuit: () => void;
};

let tray: Tray | null = null;
let reminderInterval: ReturnType<typeof setInterval> | null = null;
let reminderMinutes = 0; // 0 = off

/** Set up the system tray icon and context menu. */
export function setupTray(opts: TrayOptions): void {
  if (tray) return; // already set up

  tray = new Tray(opts.icon);
  tray.setToolTip('MedOps Offline Operations Console');
  tray.on('double-click', opts.onOpen);

  rebuildMenu(opts);
}

/** Tear down the tray on app quit. */
export function cleanupTray(): void {
  clearReminder();
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
            rebuildMenu(opts); // refresh checkmark
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

// ─── Reminder system ──────────────────────────────────────────────────────────

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
    fireReminder(opts);
  }, ms);
}

function fireReminder(opts: TrayOptions): void {
  if (!Notification.isSupported()) return;

  const n = new Notification({
    title: 'MedOps Reminder',
    body: 'Time to check outstanding work orders and low-stock alerts.',
    icon: tray ? undefined : undefined, // uses default app icon
    timeoutType: 'default',
  });

  n.on('click', opts.onOpen);
  n.show();
}
