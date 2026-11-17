package session

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/whiletrue2009/ths-favorite-go/internal/thsclient"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "session.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore 失败: %v", err)
	}
	in := &Session{
		Cookies:    map[string]string{"userid": "123", "user": "abc=="},
		AuthParams: thsclientAuthForTest(),
	}
	if err := store.Save(in); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	// 权限校验：Windows 无 POSIX 权限位，跳过。
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(path); err == nil {
			if mode := info.Mode().Perm(); mode != 0o600 {
				t.Errorf("文件权限 = %o, want 600", mode)
			}
		}
		if info, err := os.Stat(filepath.Dir(path)); err == nil {
			if mode := info.Mode().Perm(); mode != 0o700 {
				t.Errorf("目录权限 = %o, want 700", mode)
			}
		}
	}

	out, err := store.Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if out.Cookies["userid"] != "123" || out.AuthParams.UserID != "u1" {
		t.Errorf("往返数据不符: %+v", out)
	}
	if out.SavedAt == 0 {
		t.Error("SavedAt 应被填充")
	}
}

func TestLoadMissing(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "none.json"))
	if err != nil {
		t.Fatalf("NewStore 失败: %v", err)
	}
	if _, err := store.Load(); err != ErrNoSession {
		t.Errorf("期望 ErrNoSession，实际 %v", err)
	}
}

func TestLoadCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte("{{{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := NewStore(path)
	if _, err := store.Load(); err == nil || err == ErrNoSession {
		t.Errorf("损坏文件应报错并提示重新登录，实际 %v", err)
	}
}

func TestExpired(t *testing.T) {
	old := &Session{Cookies: map[string]string{"a": "b"}, SavedAt: time.Now().Add(-25 * time.Hour).Unix()}
	if !old.Expired() {
		t.Error("25 小时前的登录态应判定过期")
	}
	fresh := &Session{Cookies: map[string]string{"a": "b"}, SavedAt: time.Now().Unix()}
	if fresh.Expired() {
		t.Error("刚保存的登录态不应过期")
	}
	noStamp := &Session{Cookies: map[string]string{"a": "b"}}
	if !noStamp.Expired() {
		t.Error("无时间戳应判定过期")
	}
}

func TestClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	store, _ := NewStore(path)
	if err := store.Save(&Session{Cookies: map[string]string{"a": "b"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear 失败: %v", err)
	}
	if err := store.Clear(); err != nil {
		t.Errorf("重复 Clear 应幂等: %v", err)
	}
}

func thsclientAuthForTest() thsclient.AuthParams {
	return thsclient.AuthParams{UserID: "u1", SessionID: "s1", Expires: "2026-01-01 00:00:00"}
}
