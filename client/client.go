package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/nhirsama/Naniwosuruno/client/LinuxKDE"
	"github.com/nhirsama/Naniwosuruno/client/inter"
	"github.com/nhirsama/Naniwosuruno/pkg"
)

type UpdatePayload struct {
	Title string `json:"title"`
	OS    OSType `json:"os"`
}

type OSType string

const (
	Windows OSType = "windows"
	Linux   OSType = "linux"
	MocOS   OSType = "darwin"
	Android OSType = "android"
)

type DesktopType string

const (
	KDE DesktopType = "KDE"
)

type Client struct {
	os              OSType
	desktop         DesktopType
	handle          inter.GetWindowTitle
	http            *http.Client
	token           string
	bashURL         string
	lastWindowTitle string
}

// NewClient 初始化客户端
func NewClient() *Client {
	c := &Client{}
	c.os = OSType(runtime.GOOS)
	c.desktop = DesktopType(os.Getenv("XDG_CURRENT_DESKTOP"))
	if c.desktop == "" {
		c.desktop = DesktopType(os.Getenv("DESKTOP_SESSION"))
	}

	// 配置读取
	// 注意：如果配置文件损坏，这里可能会由 pkg.ReadConfig() 导致 fatal error
	c.token = pkg.ReadConfig().Token
	c.http = &http.Client{}
	c.bashURL = pkg.ReadConfig().BaseUrl
	if c.bashURL == "" {
		c.bashURL = "http://localhost:9975"
	}
	return c
}

// Run 启动客户端主循环
func Run() {
	c := NewClient()
	c.Start()
}

func (c *Client) Start() {
	// 检查系统支持
	switch c.os {
	case Windows:
		log.Fatal("Windows not yet supported")
	case Linux:
		switch c.desktop {
		case KDE:
			c.handle = LinuxKDE.NewWindowTitle()
		default:
			log.Fatalf("Linux %s desktop not yet supported", c.desktop)
		}
	case MocOS:
		log.Fatal("MocOS not yet supported")
	case Android:
		log.Fatal("Android not yet supported")
	default:
		log.Fatalf("%s not yet supported", c.os)
	}

	log.Printf("Client started on %s (%s)", c.os, c.desktop)

	for {
		windowTitle, err := c.handle.GetWindowTitle()
		if err != nil {
			log.Printf("GetWindowTitle failed: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if c.lastWindowTitle != windowTitle {
			c.lastWindowTitle = windowTitle
			log.Printf("WindowTitle changed: %s", windowTitle)
			c.request(c.lastWindowTitle)
		}
		time.Sleep(5 * time.Second)
	}
}

func (c *Client) request(title string) bool {
	payload := UpdatePayload{
		Title: title,
		OS:    c.os,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Println("序列化 JSON 失败:", err)
		return false
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/update", c.bashURL), bytes.NewBuffer(jsonData))
	if err != nil {
		log.Println("创建请求失败:", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "token", Value: c.token})
	resp, err := c.http.Do(req)
	if err != nil {
		log.Println("发送到服务端失败:", err)
		return false
	}
	err = resp.Body.Close()
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}
