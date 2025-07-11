package pkg

/*
读取./data/config.json中的token数据并返回
*/
import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type config struct {
	Token string
}

func generateToken() string {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		log.Fatal("生成 token 失败:", err)
	}
	return hex.EncodeToString(bytes)
}
func ReadToken() string {
	configDir := "./data"
	configPath := filepath.Join(configDir, "config.json")

	//检查文件夹是否存在
	if err := os.MkdirAll(configDir, os.ModePerm); err != nil {
		log.Fatal("无法创建 data 目录:", err)
	}
	var cfg config
	//检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg.Token = generateToken()
		date, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			log.Fatal("序列化 JSON 失败：", err)
		}

		if err := os.WriteFile(configPath, date, os.ModePerm); err != nil {
			log.Fatal("写入 JSON 文件失败:", err)
		}

		fmt.Println("文件不存在，初始化 token 为:", cfg.Token)
	} else if err != nil {
		log.Fatal("读取文件状态时出错:", err)
	} else {
		data, err := os.ReadFile(configPath)
		if err != nil {
			log.Fatal("读取文件失败:", err)
		}

		if err := json.Unmarshal(data, &cfg); err != nil {
			log.Fatal("解析 JSON 失败:", err)
		}

		if len(cfg.Token) < 32 || len(cfg.Token) > 32 {
			fmt.Println(len(cfg.Token), cfg.Token)
			cfg.Token = generateToken()
			date, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				log.Fatal("序列化 JSON 失败：", err)
			}

			if err := os.WriteFile(configPath, date, os.ModePerm); err != nil {
				log.Fatal("写入 JSON 文件失败:", err)
			}
			fmt.Println("token 格式非法，重新设置 token 为：", cfg.Token)
		} else {
			log.Println("正确读取到 token ")
		}
	}
	return cfg.Token
}
