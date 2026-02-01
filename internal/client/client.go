package client

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/nhirsama/Naniwosuruno/internal/client/LinuxKDE"
	clientWindows "github.com/nhirsama/Naniwosuruno/internal/client/Windows"
	"github.com/nhirsama/Naniwosuruno/internal/client/inter"
	"github.com/nhirsama/Naniwosuruno/pkg"
)

type OSType string
type DesktopType string

const (
	Windows OSType = "windows"
	Linux   OSType = "linux"
	MacOS   OSType = "darwin"
	Android OSType = "android"

	KDE DesktopType = "KDE"
)

type Client struct {
	os              OSType
	desktop         DesktopType
	handle          inter.GetWindowTitle
	config          *pkg.AppConfig
	connection      *ServerConnection
	lastWindowTitle string
}

func Run() {
	NewClient().Start()
}

func NewClient() *Client {
	c := &Client{
		os:      OSType(runtime.GOOS),
		desktop: detectDesktop(),
		config:  pkg.ReadConfig(),
	}

	c.ensureKeys()

	c.connection = NewServerConnection(c.config)

	return c
}

func (c *Client) Start() {
	c.initWindowHandle()
	log.Printf("Client started on %s (%s)", c.os, c.desktop)

	c.connection.Connect()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.checkAndUpdateWindowTitle()
	}
}

// --- Initialization Helpers ---

func detectDesktop() DesktopType {
	if d := os.Getenv("XDG_CURRENT_DESKTOP"); d != "" {
		return DesktopType(d)
	}
	return DesktopType(os.Getenv("DESKTOP_SESSION"))
}

func (c *Client) ensureKeys() {
	if c.config.PrivateKey != "" {
		return
	}

	log.Println("未配置私钥，生成新的密钥对...")
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		log.Fatalf("生成密钥失败: %v", err)
	}

	c.config.PrivateKey = base64.StdEncoding.EncodeToString(privKey)
	if c.config.ClientID == "" {
		c.config.ClientID = uuid.New().String()
	}
	pubKeyStr := base64.StdEncoding.EncodeToString(pubKey)

	// 将自己添加到信任列表 (针对本地单机部署场景)
	found := false
	for _, client := range c.config.Clients {
		if client.ID == c.config.ClientID {
			found = true
			break
		}
	}
	if !found {
		name, _ := os.Hostname()
		if name == "" {
			name = "LocalClient"
		}
		c.config.Clients = append(c.config.Clients, pkg.ClientConfig{
			ID:        c.config.ClientID,
			Name:      name,
			PublicKey: pubKeyStr,
		})
		log.Println("已自动将新生成的公钥添加到信任客户端列表")
	}

	if err := pkg.SaveConfig(c.config); err != nil {
		log.Fatalf("保存配置失败: %v", err)
	}

	fmt.Println("==================================================")
	fmt.Printf("新密钥已生成！\n")
	fmt.Printf("Client ID: %s\n", c.config.ClientID)
	fmt.Printf("Public Key: %s\n", pubKeyStr)
	fmt.Println("==================================================")
}

func (c *Client) initWindowHandle() {
	switch c.os {
	case Windows:
		c.handle = clientWindows.NewWindowTitle()
	case Linux:
		if c.desktop == KDE {
			c.handle = LinuxKDE.NewWindowTitle()
		} else {
			log.Fatalf("Linux desktop '%s' not supported", c.desktop)
		}
	default:
		log.Fatalf("OS '%s' not supported", c.os)
	}
}

// --- Logic ---

func (c *Client) checkAndUpdateWindowTitle() {
	title, err := c.handle.GetWindowTitle()
	if err != nil {
		log.Printf("获取窗口标题失败: %v", err)
		return
	}

	if c.lastWindowTitle != title {
		log.Printf("标题变更: %s", title)

		payload := &UpdatePayload{
			Title: title,
			OS:    c.os,
		}

		if err := c.connection.SendUpdate(payload); err != nil {
			log.Printf("发送更新失败: %v", err)
			return
		}

		c.lastWindowTitle = title
	}
}
