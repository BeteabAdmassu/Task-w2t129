/**
 * electron-builder configuration — MedOps Offline Operations Console
 * Produces Windows NSIS (.exe) and MSI installers.
 */

'use strict';

/** @type {import('electron-builder').Configuration} */
const config = {
  appId: 'com.medops.console',
  productName: 'MedOps Console',
  copyright: 'MedOps Healthcare Systems',

  directories: {
    output: 'dist-installer',
    buildResources: 'build-resources',
  },

  // Files included in the app package
  files: [
    'dist/**/*',        // compiled React SPA
    'dist-main/**/*',   // compiled Electron main + preload
    'package.json',
  ],

  // Extra resources bundled alongside the app (not in asar)
  extraResources: [
    {
      from: '../backend-dist/',   // pre-built Go binaries (see scripts/build-backend.sh)
      to: 'backend',
      filter: ['**/*'],
    },
  ],

  // ── Windows ─────────────────────────────────────────────────────────────────
  win: {
    target: [
      { target: 'nsis', arch: ['x64'] },
      { target: 'msi',  arch: ['x64'] },
    ],
    icon: 'build-resources/icon.ico',
    publisherName: 'MedOps Healthcare Systems',
    signingHashAlgorithms: ['sha256'],
  },

  // PostgreSQL is bundled via embedded-postgres — no separate install required
  // NSIS installer (one-time setup wizard)
  nsis: {
    oneClick: false,
    allowToChangeInstallationDirectory: true,
    allowElevation: true,
    installerIcon: 'build-resources/icon.ico',
    uninstallerIcon: 'build-resources/icon.ico',
    installerHeader: 'build-resources/installer-header.bmp',
    shortcutName: 'MedOps Console',
    createDesktopShortcut: true,
    createStartMenuShortcut: true,
  },

  // MSI installer
  msi: {
    oneClick: false,
    perMachine: true,
    createDesktopShortcut: true,
    createStartMenuShortcut: true,
  },

  // ── macOS (future target) ────────────────────────────────────────────────────
  mac: {
    target: 'dmg',
    category: 'public.app-category.medical',
  },

  // ── Linux (future target) ────────────────────────────────────────────────────
  linux: {
    target: 'AppImage',
    category: 'MedicalSoftware',
  },
};

module.exports = config;
