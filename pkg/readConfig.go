package pkg

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// AppConfig 存放应用配置
type AppConfig struct {
	Token   string `json:"Token"`
	BaseUrl string `json:"BaseUrl"`
}

// ConfigLoader 定义加载配置的接口
type ConfigLoader interface {
	Load() (*AppConfig, error)
}

// JSONConfigLoader 实现从 JSON 文件加载配置
type JSONConfigLoader struct {
	DataDir  string
	FileName string
}

// NewJSONConfigLoader 创建一个新的 JSONConfigLoader
func NewJSONConfigLoader() *JSONConfigLoader {
	return &JSONConfigLoader{
		DataDir:  "./data",
		FileName: "config.json",
	}
}

// Load 实现 ConfigLoader 接口
func (l *JSONConfigLoader) Load() (*AppConfig, error) {
	configPath := filepath.Join(l.DataDir, l.FileName)

	// 确保目录存在
	if err := os.MkdirAll(l.DataDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("无法创建目录 %s: %w", l.DataDir, err)
	}

	cfg := &AppConfig{}

	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 文件不存在，初始化
		cfg.Token = generateToken()
		if err := l.save(cfg, configPath); err != nil {
			return nil, err
		}
		fmt.Println("配置文件不存在，已初始化并生成 Token: ", cfg.Token)
	} else if err != nil {
		return nil, fmt.Errorf("检查文件状态出错: %w", err)
	} else {
		// 文件存在，读取
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}

		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("解析 JSON 失败: %w", err)
		}

		// 验证 Token
		if len(cfg.Token) != 32 {
			fmt.Printf("Token 格式非法 (长度 %d)，重新生成...\n", len(cfg.Token))
			cfg.Token = generateToken()
			if err := l.save(cfg, configPath); err != nil {
				return nil, err
			}
			fmt.Println("已重新生成 Token: ", cfg.Token)
		} else {
			//log.Println("成功加载配置")
		}
	}

	return cfg, nil
}

// save 辅助方法：保存配置到文件
func (l *JSONConfigLoader) save(cfg *AppConfig, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.WriteFile(path, data, os.ModePerm); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}

// generateToken 生成随机 Token
func generateToken() string {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		log.Fatal("生成 Token 失败:", err)
	}
	return hex.EncodeToString(bytes)
}

// ReadConfig 是一个便捷函数，用于保持一定程度的易用性，
// 但现在它返回 *AppConfig 结构体而不是字符串。
// 注意：这更改了原有 ReadToken 的签名。
func ReadConfig() *AppConfig {
	loader := NewJSONConfigLoader()
	cfg, err := loader.Load()
	if err != nil {
		log.Fatal("加载配置失败:", err)
	}
	return cfg
}
