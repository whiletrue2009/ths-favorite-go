// Package session 管理本地登录态（Cookie 缓存）。
// 文件权限 0600、目录 0700，默认放在 OS 用户配置目录。
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/whiletrue2009/ths-favorite-go/internal/thsclient"
)

// CookieTTL 与官方会话有效期对齐：超过即视为过期，需要重新登录。
const CookieTTL = 24 * time.Hour

// Session 是持久化的登录态快照。
type Session struct {
	Cookies    map[string]string    `json:"cookies"`
	AuthParams thsclient.AuthParams `json:"auth_params"`
	Username   string               `json:"username,omitempty"`
	SavedAt    int64                `json:"saved_at"`
}

// Store 负责 session.json 的读写。
type Store struct {
	path string
}

// NewStore 创建存储；path 为空时使用默认路径。
func NewStore(path string) (*Store, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	return &Store{path: path}, nil
}

// DefaultPath 返回 OS 用户配置目录下的 session.json。
func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("无法确定用户配置目录: %w", err)
	}
	return filepath.Join(configDir, "ths", "session.json"), nil
}

// Path 返回存储路径。
func (s *Store) Path() string { return s.path }

// ErrNoSession 表示本地没有登录态。
var ErrNoSession = errors.New("本地没有已保存的登录态")

// Load 读取登录态；文件不存在返回 ErrNoSession，文件损坏同样视为无登录态（并提示）。
func (s *Store) Load() (*Session, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSession
		}
		return nil, fmt.Errorf("读取登录态失败: %w", err)
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("登录态文件损坏（%s），请重新运行 ths login: %w", s.path, err)
	}
	if len(session.Cookies) == 0 {
		return nil, ErrNoSession
	}
	return &session, nil
}

// Save 以 0600 权限写入登录态，目录以 0700 创建。
func (s *Store) Save(session *Session) error {
	session.SavedAt = time.Now().Unix()
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化登录态失败: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("创建状态目录失败: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("设置状态目录权限失败: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("写入登录态失败: %w", err)
	}
	return nil
}

// Clear 删除登录态文件（登出）。
func (s *Store) Clear() error {
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除登录态失败: %w", err)
	}
	return nil
}

// Age 返回登录态已保存的时长；无时间戳视为极旧。
func (s *Session) Age() time.Duration {
	if s.SavedAt == 0 {
		return time.Duration(math.MaxInt64)
	}
	return time.Since(time.Unix(s.SavedAt, 0))
}

// Expired 判断登录态是否超过 TTL。
func (s *Session) Expired() bool {
	return s.Age() > CookieTTL
}
