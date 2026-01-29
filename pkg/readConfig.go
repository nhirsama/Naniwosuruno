package pkg

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// AppConfig 存储应用程序的所有配置项，包括 Token、BaseUrl 以及安全认证所需的密钥和客户端列表
type AppConfig struct {
	Token      string         `json:"Token"`
	BaseUrl    string         `json:"BaseUrl"`
	ClientID   string         `json:"ClientID,omitempty"`   // 客户端用于标识自身身份的 ID
	PrivateKey string         `json:"PrivateKey,omitempty"` // 客户端用于签名的 Ed25519 私钥 (Base64)
	Clients    []ClientConfig `json:"Clients,omitempty"`    // 服务端信任的客户端列表 (包含公钥)
}

// ClientConfig 定义了服务端所知的客户端元数据，包括用于验签的公钥
type ClientConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"` // Ed25519 公钥 (Base64)
}

// ConfigLoader 定义了加载和保存配置的底层行为，支持未来可能的多种格式（如 YAML/ETCD）
type ConfigLoader interface {
	Load() (*AppConfig, error)
	Save(*AppConfig) error
}

// JSONConfigLoader 实现了基于本地 JSON 文件的配置加载
type JSONConfigLoader struct {
	DataDir  string
	FileName string
}

func NewJSONConfigLoader() *JSONConfigLoader {
	return &JSONConfigLoader{
		DataDir:  "./data",
		FileName: "config.json",
	}
}

// Load 从磁盘读取 JSON 配置，如果文件缺失则触发初始化逻辑
func (l *JSONConfigLoader) Load() (*AppConfig, error) {
	configPath := filepath.Join(l.DataDir, l.FileName)

	if err := os.MkdirAll(l.DataDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("无法创建配置目录: %w", err)
	}

	cfg := &AppConfig{}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return l.initializeConfig(cfg, configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	// 兼容性检查：确保 Token 存在且格式基本正确
	if len(cfg.Token) != 32 {
		log.Printf("Token 格式非法，重新生成...")
		cfg.Token = generateToken()
		if err := l.Save(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func (l *JSONConfigLoader) initializeConfig(cfg *AppConfig, path string) (*AppConfig, error) {
	cfg.Token = generateToken()
	if err := l.Save(cfg); err != nil {
		return nil, err
	}
	log.Printf("配置文件已初始化: %s", path)
	return cfg, nil
}

// Save 将配置对象持久化到 JSON 文件
func (l *JSONConfigLoader) Save(cfg *AppConfig) error {
	configPath := filepath.Join(l.DataDir, l.FileName)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	return os.WriteFile(configPath, data, os.ModePerm)
}

func generateToken() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatal("生成 Token 失败:", err)
	}
	return hex.EncodeToString(bytes)
}

// ConfigManager 提供了线程安全的配置管理功能，支持在运行时重载磁盘配置（热更新）
type ConfigManager struct {
	loader    ConfigLoader
	config    *AppConfig
	clientMap map[string]ClientConfig // ID -> ClientConfig 的索引，用于提高服务端查询公钥的速度
	mu        sync.RWMutex
}

func NewConfigManager() (*ConfigManager, error) {
	loader := NewJSONConfigLoader()
	cfg, err := loader.Load()
	if err != nil {
		return nil, err
	}

	cm := &ConfigManager{
		loader: loader,
		config: cfg,
	}
	cm.rebuildIndex()
	return cm, nil
}

// rebuildIndex 将配置列表转换为 Map 结构，优化 O(n) 为 O(1) 的查找性能
func (m *ConfigManager) rebuildIndex() {
	newMap := make(map[string]ClientConfig)
	if m.config != nil {
		for _, c := range m.config.Clients {
			newMap[c.ID] = c
		}
	}
	m.clientMap = newMap
}

func (m *ConfigManager) GetConfig() *AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

func (m *ConfigManager) GetClient(id string) (ClientConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.clientMap[id]
	return c, ok
}

// Reload 强制重新加载磁盘上的配置文件，并更新内存索引
func (m *ConfigManager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, err := m.loader.Load()
	if err != nil {
		return err
	}

	m.config = cfg
	m.rebuildIndex()
	return nil
}

// 全局便捷函数，适用于客户端或不需要长期管理配置的简单组件
func ReadConfig() *AppConfig {
	loader := NewJSONConfigLoader()
	cfg, err := loader.Load()
	if err != nil {
		log.Fatal("加载配置失败:", err)
	}
	return cfg
}

func SaveConfig(cfg *AppConfig) error {
	return NewJSONConfigLoader().Save(cfg)
}
