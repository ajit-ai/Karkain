package pm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// AuthToken stores authentication credentials for the registry.
type AuthToken struct {
	Username string `json:"username"`
	Token    string `json:"token"`
}

func authConfigPath() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("USERPROFILE")
		if home == "" {
			home = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		}
		return filepath.Join(home, ".karkain", "auth.json")
	}
	home := os.Getenv("HOME")
	return filepath.Join(home, ".karkain", "auth.json")
}

// SaveToken persists the auth token to disk.
func SaveToken(token *AuthToken) error {
	path := authConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadToken reads the auth token from disk.
func LoadToken() (*AuthToken, error) {
	path := authConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("not logged in (no auth token found at %s)", path)
	}
	var token AuthToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("corrupted auth token: %w", err)
	}
	if token.Token == "" {
		return nil, fmt.Errorf("empty auth token")
	}
	return &token, nil
}

// ClearToken removes the auth token from disk.
func ClearToken() error {
	path := authConfigPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not remove auth token: %w", err)
	}
	return nil
}

// Authenticate sends credentials to the registry and returns a token.
func Authenticate(username, password string) (*AuthToken, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password required")
	}

	c := NewRegistryClient()
	// TODO: implement real authentication against registry API
	// For now, create a local-only token
	token := &AuthToken{
		Username: username,
		Token:    "local-dev-token-" + username,
	}

	_ = c
	_ = password
	return token, nil
}
