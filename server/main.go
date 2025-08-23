package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/nhirsama/Naniwosuruno/pkg"
	"github.com/r3labs/sse/v2"
)

var token string

func init() {
	token = pkg.ReadToken()
	//fmt.Println(token)
}

func main() {
	sseServer := sse.New()

	// 把每个 Stream 的队列长度从 1024 缩到 64
	sseServer.BufferSize = 64

	sseServer.CreateStream("focus") // 创建 "focus" 流
	http.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "方法不允许，请使用 POST", http.StatusMethodNotAllowed)
			return
		}
		//fmt.Println("请求头：", r.Header)

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

		// 在最后关闭请求体
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				log.Println("关闭请求体失败", err)
			}
		}(r.Body)

		if err != nil {
			log.Println("读取请求体时错误:", err)
			return
		}

		// 发布到 SSE 流
		ok := sseServer.TryPublish("focus", &sse.Event{Data: body})
		if !ok {
			log.Printf("队列已满，丢弃一次更新: %s\n", string(body))
		}
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
	if err != nil {
		log.Fatal("服务启动失败", err)
		return
	}
}
