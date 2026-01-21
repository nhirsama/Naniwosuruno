package main

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

var client Client

func init() {
	client.os = OSType(runtime.GOOS)
	client.desktop = DesktopType(os.Getenv("XDG_CURRENT_DESKTOP"))
	if client.desktop == "" {
		client.desktop = DesktopType(os.Getenv("DESKTOP_SESSION"))
	}
	switch client.os {
	case Windows:
		panic("Windows not yet supported")
	case Linux:
		switch client.desktop {
		case KDE:
			client.handle = LinuxKDE.NewWindowTitle()
		default:
			panic(fmt.Sprintf("Linux %s desktop not yet supported", client.os))
		}
	case MocOS:
		panic("MocOS not yet supported")
	case Android:
		panic("Android not yet supported")
	default:
		panic(fmt.Sprintf("%s not yet supported", client.os))
	}

	client.token = pkg.ReadToken()
	client.http = &http.Client{}
	client.bashURL = "http://localhost:9975"
}

func main() {
	client.Run()
}

func (c *Client) Run() {
	for {
		windowTitle, err := c.handle.GetWindowTitle()
		if err != nil {
			log.Printf("GetWindowTitle failed: %v", err)
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
	} else {
		err := resp.Body.Close()
		if err != nil {
			log.Println(err)
			return false
		}
	}
	return true
}
