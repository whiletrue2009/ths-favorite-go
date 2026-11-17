package creds

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// newFileBackend 构造钥匙串不可用、走文件回退的存储。
func newFileBackend(t *testing.T) *Storage {
	t.Helper()
	return &Storage{
		keyringErr:   errors.New("keyring unavailable"),
		fallbackPath: filepath.Join(t.TempDir(), "credentials.json"),
	}
}

func TestFileBackendRoundtrip(t *testing.T) {
	storage := newFileBackend(t)
	if _, err := storage.Load(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("初始应为 ErrNotFound: %v", err)
	}
	if err := storage.Save(&Credentials{Username: "13800000000", Password: "pw"}); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	// 文件权限校验（非 Windows）。
	if info, err := os.Stat(storage.fallbackPath); err == nil && info.Mode().Perm() != 0o600 {
		t.Errorf("文件权限 = %o, want 600", info.Mode().Perm())
	}
	loaded, err := storage.Load()
	if err != nil || loaded.Username != "13800000000" || loaded.Password != "pw" {
		t.Fatalf("Load 不符: %+v %v", loaded, err)
	}
	if err := storage.Delete(); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if _, err := storage.Load(); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除后应为 ErrNotFound: %v", err)
	}
}

func TestFileBackendCorruptFile(t *testing.T) {
	storage := newFileBackend(t)
	if err := os.MkdirAll(filepath.Dir(storage.fallbackPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storage.fallbackPath, []byte("{{{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Load(); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("损坏文件应报错: %v", err)
	}
}

func TestDecodeCredentialsIncomplete(t *testing.T) {
	if _, err := decodeCredentials([]byte(`{"username":"a"}`)); err == nil {
		t.Error("缺密码应报错")
	}
}
