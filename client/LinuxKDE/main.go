package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/nhirsama/Naniwosuruno/pkg"
)

var (
	token   string
	client  *http.Client
	bashURL string = "http://localhost:9975"
)

func init() {
	token = pkg.ReadToken()
	client = &http.Client{}

}
func getWindowsName() (string, bool) {
	cmd := exec.Command("./kdotool", "getactivewindow", "getwindowname")
	out, err := cmd.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			log.Printf("程序退出码为：%d\n", exitError.ExitCode())
		}
		log.Println(err)
		return "", false
	}
	outStr := strings.TrimSpace(string(out))
	return outStr, true
}

func request(title string) bool {
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/date", bashURL), bytes.NewBuffer([]byte(title)))
	if err != nil {
		log.Println("创建请求失败:", err)
		return false
	}
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := client.Do(req)
	if err != nil {
		log.Println("发送到服务端失败:", err)
		return false
	} else {
		resp.Body.Close()
	}
	return true
}
func main() {
	var lastTitle string
	for {
		windowName, ok := getWindowsName()
		if ok {
			cleanWindowName := pkg.CleanWindowTitle(windowName)
			if lastTitle != cleanWindowName {
				log.Printf("窗口名发生变动，清洗前窗口名为：%s, 清洗后窗口名为：%s\n", windowName, cleanWindowName)
				lastTitle = cleanWindowName
				request(cleanWindowName)
			}
		} else {
			log.Println("窗口名称获取失败")
		}
		time.Sleep(5 * time.Second)
	}
}
