package client

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/nhirsama/Naniwosuruno/client/LinuxKDE"
	clientWindows "github.com/nhirsama/Naniwosuruno/client/Windows"
	"github.com/nhirsama/Naniwosuruno/client/inter"
	"github.com/nhirsama/Naniwosuruno/pkg"
	"github.com/nhirsama/Naniwosuruno/pkg/auth"
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
	http            *http.Client
	config          *pkg.AppConfig
	token           string
	bashURL         string
	authenticator   auth.ClientAuthenticator
	useV1           bool
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
		http:    &http.Client{Timeout: 10 * time.Second},
	}

	c.ensureKeys()
	c.initAuthenticator()
	c.configureBaseURL()

	c.token = c.config.Token // 默认使用静态 Token

	return c
}

func (c *Client) Start() {
	c.initWindowHandle()
	log.Printf("Client started on %s (%s)", c.os, c.desktop)

	c.performHandshake()

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

func (c *Client) initAuthenticator() {
	if c.config.PrivateKey == "" {
		return
	}
	auth, err := auth.NewClientAuthenticatorFromBase64(c.config.PrivateKey)
	if err != nil {
		log.Printf("认证器初始化失败: %v", err)
		return
	}
	c.authenticator = auth
}

func (c *Client) configureBaseURL() {
	c.bashURL = c.config.BaseUrl
	if c.bashURL == "" {
		c.bashURL = "http://localhost:9975"
	}
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

func (c *Client) performHandshake() {
	if c.authenticator == nil || c.config.ClientID == "" {
		log.Println("跳过握手，使用 API v0 (Static Token)")
		c.useV1 = false
		return
	}

	if err := c.authenticateV1(); err != nil {
		log.Printf("认证失败: %v, 回退到 API v0", err)
		c.useV1 = false
	} else {
		log.Println("认证成功，使用 API v1")
		c.useV1 = true
	}
}

func (c *Client) checkAndUpdateWindowTitle() {
	title, err := c.handle.GetWindowTitle()
	if err != nil {
		log.Printf("获取窗口标题失败: %v", err)
		return
	}

	if c.lastWindowTitle != title {
		log.Printf("标题变更: %s", title)
		c.lastWindowTitle = title
		c.sendUpdate(title)
	}
}

func (c *Client) authenticateV1() error {
	// 1. Get Challenge
	reqBody, _ := json.Marshal(auth.ChallengeRequest{ClientID: c.config.ClientID})
	resp, err := c.http.Post(c.bashURL+"/api/v1/auth/challenge", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	var challengeResp auth.ChallengeResponse
	if err := json.NewDecoder(resp.Body).Decode(&challengeResp); err != nil {
		return err
	}

	// 2. Sign
	sig, err := c.authenticator.SignChallenge(challengeResp.Challenge)
	if err != nil {
		return err
	}

	// 3. Verify
	verifyReq, _ := json.Marshal(auth.VerifyRequest{
		ClientID:  c.config.ClientID,
		Signature: sig,
	})
	resp, err = c.http.Post(c.bashURL+"/api/v1/auth/verify", "application/json", bytes.NewBuffer(verifyReq))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("verify status %d", resp.StatusCode)
	}

	var verifyResp auth.VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		return err
	}

	c.token = verifyResp.Token
	return nil
}

func (c *Client) sendUpdate(title string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"title": title,
		"os":    c.os,
	})

	path := "/api/v0/update"
	if c.useV1 {
		path = "/api/v1/update"
	}

	req, err := http.NewRequest("POST", c.bashURL+path, bytes.NewBuffer(payload))
	if err != nil {
		log.Println("创建请求失败:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "token", Value: c.token})

	resp, err := c.http.Do(req)
	if err != nil {
		log.Println("发送失败:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("服务端错误: %d", resp.StatusCode)
		if c.useV1 && resp.StatusCode == http.StatusUnauthorized {
			log.Println("Session 可能已过期")
		}
		log.Println("尝试重新建立 SSE 连接")
		c.initAuthenticator()
	}
}
