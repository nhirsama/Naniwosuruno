package main

import (
	"Naniwosuruno/server/pkg"
	"fmt"
	"github.com/r3labs/sse/v2"
	"io"
	"log"
	"net/http"
)

var token string

func init() {
	token = pkg.ReadToken()
	fmt.Println(token)
}

func main() {
	sseServer := sse.New()
	http.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "方法不允许，请使用 POST", http.StatusMethodNotAllowed)
			return
		}

		// 检查 Cookie
		cookie, err := r.Cookie("token")
		if err != nil {
			http.Error(w, "未授权：无效的cookie", http.StatusUnauthorized)
			fmt.Println(err)
			return
		} else if cookie.Value != token {
			http.Error(w, "未授权：无效的token", http.StatusUnauthorized)
			return
		}
		// 读取请求体
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println("读取请求体时错误:", err)
			return
		}

		// 发布到 SSE 流
		sseServer.Publish("focus", &sse.Event{Data: body})
		_, err = fmt.Fprintf(w, "收到标题: %s", string(body))
		if err != nil {
			return
		}
	})

	// 处理 SSE 连接
	http.HandleFunc("/events", sseServer.ServeHTTP)

	// 提供 Web 页面
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// 启动服务
	fmt.Println("服务端启动于 :9975")
	err := http.ListenAndServe(":9975", nil)
	for err != nil {
		log.Fatal("服务启动失败", err)
		return
	}
}
