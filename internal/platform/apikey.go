package platform

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CLIClientID is the gateway client id of the key `aibox` itself uses (smoke, bench).
const CLIClientID = "aibox-cli"

// APIKeySecret holds the gateway's API keys (client id -> key) in the gateway namespace.
const APIKeySecret = "aibox-api-keys" //nolint:gosec // Secret name, not a credential

// APIKeyFile is where the CLI keeps its gateway key, per profile.
func APIKeyFile(stateDir string) string { return filepath.Join(stateDir, "api-key") }

// LoadOrCreateAPIKey returns the CLI's gateway key from stateDir, generating it on first use
// (32 random bytes, hex, file mode 0600).
func LoadOrCreateAPIKey(stateDir string) (string, error) {
	path := APIKeyFile(stateDir)
	data, err := os.ReadFile(path) //nolint:gosec // generated file
	if err == nil {
		if key := strings.TrimSpace(string(data)); key != "" {
			return key, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	key := "aibox-" + hex.EncodeToString(b)
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(key+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("save API key: %w", err)
	}
	return key, nil
}
