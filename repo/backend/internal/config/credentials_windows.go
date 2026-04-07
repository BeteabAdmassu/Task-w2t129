//go:build windows

package config

// Windows Credential Manager integration.
// Reads and writes secrets through the OS-managed credential vault
// (Control Panel → Credential Manager → Windows Credentials).
//
// When running as a packaged Electron desktop app the Electron main process
// should call `StoreDesktopCredentials` once (e.g., on first-run wizard) to
// persist the secrets.  Subsequent starts load them from the vault without
// ever hitting env-vars or config files.

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	modAdvapi32   = syscall.NewLazyDLL("advapi32.dll")
	procCredRead  = modAdvapi32.NewProc("CredReadW")
	procCredWrite = modAdvapi32.NewProc("CredWriteW")
	procCredFree  = modAdvapi32.NewProc("CredFree")
)

const (
	credTypeGeneric        = 1
	credPersistLocalMachine = 2
)

// nativeCREDENTIAL mirrors the Windows CREDENTIALW structure.
type nativeCREDENTIAL struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWrittenLow     uint32
	LastWrittenHigh    uint32
	CredentialBlobSize uint32
	CredentialBlob     uintptr
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// loadFromCredentialStore reads a generic credential from Windows Credential
// Manager. Returns ("", nil) when the credential does not exist so callers
// fall through to the env-var fallback gracefully.
func loadFromCredentialStore(targetName string) (string, error) {
	name, err := syscall.UTF16PtrFromString(targetName)
	if err != nil {
		return "", fmt.Errorf("credential store: UTF16 encode target %q: %w", targetName, err)
	}

	var pcred uintptr
	r, _, _ := procCredRead.Call(
		uintptr(unsafe.Pointer(name)),
		credTypeGeneric,
		0,
		uintptr(unsafe.Pointer(&pcred)),
	)
	if r == 0 {
		// Credential not found — not an error, caller falls back to env var.
		return "", nil
	}
	defer procCredFree.Call(pcred)

	cred := (*nativeCREDENTIAL)(unsafe.Pointer(pcred))
	if cred.CredentialBlobSize == 0 || cred.CredentialBlob == 0 {
		return "", nil
	}

	blob := make([]byte, cred.CredentialBlobSize)
	for i := range blob {
		blob[i] = *(*byte)(unsafe.Pointer(cred.CredentialBlob + uintptr(i)))
	}
	return string(blob), nil
}

// StoreDesktopCredentials persists the three security-sensitive secrets in the
// Windows Credential Manager under the "MedOps/" namespace.  Call this once
// during the first-run setup wizard or an administrative bootstrap step.
func StoreDesktopCredentials(jwtSecret, encryptKey, hmacKey string) error {
	entries := map[string]string{
		"MedOps/JWTSecret":  jwtSecret,
		"MedOps/EncryptKey": encryptKey,
		"MedOps/HMACKey":    hmacKey,
	}
	for target, secret := range entries {
		if err := storeCredential(target, secret); err != nil {
			return fmt.Errorf("store %q: %w", target, err)
		}
	}
	return nil
}

func storeCredential(targetName, secret string) error {
	name, err := syscall.UTF16PtrFromString(targetName)
	if err != nil {
		return fmt.Errorf("UTF16 encode: %w", err)
	}
	blob := []byte(secret)
	if len(blob) == 0 {
		return fmt.Errorf("secret must not be empty")
	}
	cred := nativeCREDENTIAL{
		Type:               credTypeGeneric,
		TargetName:         name,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     uintptr(unsafe.Pointer(&blob[0])),
		Persist:            credPersistLocalMachine,
	}
	r, _, e := procCredWrite.Call(uintptr(unsafe.Pointer(&cred)), 0)
	if r == 0 {
		return fmt.Errorf("CredWriteW: %w", e)
	}
	return nil
}
