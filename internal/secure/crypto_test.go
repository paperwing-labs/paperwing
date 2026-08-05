package secure

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	c, err := New(bytes.Repeat([]byte{42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := c.Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	if string(encrypted) == "secret" {
		t.Fatal("password was stored as plaintext")
	}
	got, err := c.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if got != "secret" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadOrCreateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "master.key")
	first, created, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if !created || len(first) != 32 {
		t.Fatalf("created=%v length=%d", created, len(first))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions=%o", info.Mode().Perm())
	}
	second, created, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if created || !bytes.Equal(first, second) {
		t.Fatal("existing key was not reused")
	}
}

func TestLoadOrCreateKeyRejectsInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "master.key")
	if err := os.WriteFile(path, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadOrCreateKey(path); err == nil {
		t.Fatal("expected invalid key file error")
	}
}
