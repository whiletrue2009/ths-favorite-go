// Package creds 管理同花顺账号凭据的本地保存。
// 首选 OS 钥匙串（macOS Keychain / Windows 凭据管理器）；
// 钥匙串不可用时回退到用户配置目录下的 0600 JSON 文件。
// 凭据永远不进命令行参数、不进环境变量、不进日志。
package creds

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "ths-favorite-go"
	keyringUser    = "default"
)

// Credentials 是保存的账号密码。
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Storage 提供凭据的读写；Backend 描述实际生效的存储位置。
type Storage struct {
	// keyringErr 非 nil 表示钥匙串初始化失败，回退文件模式。
	keyringErr   error
	fallbackPath string
}

// New 构造存储。用一次只读探测判断钥匙串是否可用：
// 能命中「条目不存在」说明钥匙串服务正常应答，仅无数据。
func New() *Storage {
	s := &Storage{fallbackPath: defaultFallbackPath()}
	_, err := keyring.Get(keyringService, keyringUser)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return s
	}
	s.keyringErr = err
	return s
}

// Backend 返回凭据实际存放位置说明（doctor 输出用）。
func (s *Storage) Backend() string {
	if s.keyringErr == nil {
		return "OS 钥匙串"
	}
	return "本地文件（钥匙串不可用: " + s.keyringErr.Error() + "）"
}

// ErrNotFound 表示没有保存的凭据。
var ErrNotFound = errors.New("没有已保存的账号凭据")

// Load 读取凭据：钥匙串优先，回退文件。
func (s *Storage) Load() (*Credentials, error) {
	if s.keyringErr == nil {
		value, err := keyring.Get(keyringService, keyringUser)
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("读取钥匙串失败: %w", err)
		}
		return decodeCredentials([]byte(value))
	}
	return s.loadFallback()
}

// Save 写入凭据。
func (s *Storage) Save(cred *Credentials) error {
	encoded, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("序列化凭据失败: %w", err)
	}
	if s.keyringErr == nil {
		if err := keyring.Set(keyringService, keyringUser, string(encoded)); err != nil {
			return fmt.Errorf("写入钥匙串失败: %w", err)
		}
		return nil
	}
	return s.saveFallback(encoded)
}

// Delete 删除凭据（登出）。
func (s *Storage) Delete() error {
	if s.keyringErr == nil {
		if err := keyring.Delete(keyringService, keyringUser); err != nil {
			if errors.Is(err, keyring.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("删除钥匙串条目失败: %w", err)
		}
		return nil
	}
	if err := os.Remove(s.fallbackPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除凭据文件失败: %w", err)
	}
	return nil
}

func defaultFallbackPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "credentials.json"
	}
	return filepath.Join(configDir, "ths", "credentials.json")
}

func (s *Storage) loadFallback() (*Credentials, error) {
	data, err := os.ReadFile(s.fallbackPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("读取凭据文件失败: %w", err)
	}
	return decodeCredentials(data)
}

func (s *Storage) saveFallback(encoded []byte) error {
	dir := filepath.Dir(s.fallbackPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("创建凭据目录失败: %w", err)
	}
	_ = os.Chmod(dir, 0o700)
	if err := os.WriteFile(s.fallbackPath, encoded, 0o600); err != nil {
		return fmt.Errorf("写入凭据文件失败: %w", err)
	}
	return nil
}

func decodeCredentials(data []byte) (*Credentials, error) {
	var cred Credentials
	if err := json.Unmarshal(data, &cred); err != nil {
		return nil, fmt.Errorf("凭据数据损坏: %w", err)
	}
	if cred.Username == "" || cred.Password == "" {
		return nil, fmt.Errorf("凭据数据不完整")
	}
	return &cred, nil
}
