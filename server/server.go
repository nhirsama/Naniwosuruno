package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/nhirsama/Naniwosuruno/pkg"
	"github.com/r3labs/sse/v2"
)

type Server struct {
	token string
}

func Run() {
	server := NewServer()
	server.Run()
}
func NewServer() *Server {
	return &Server{
		token: pkg.ReadConfig().Token,
	}
}
func (s *Server) Run() {
	sseServer := sse.New()
	// 设置消息有效期为一天
	sseServer.EventTTL = 24 * time.Hour
	sseServer.BufferSize = 4

	sseServer.CreateStream("focus") // 创建 "focus" 流
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
		} else if cookie.Value != s.token {
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
		var payload string
		osType := r.URL.Query().Get("os")
		if osType != "" {
			payload = fmt.Sprintf(`{"title": "%s", "os": "%s"}`, string(body), osType)
		} else {
			payload = string(body)
		}

		ok := sseServer.TryPublish("focus", &sse.Event{Data: []byte(payload)})
		if !ok {
			log.Printf("队列已满，丢弃一次更新: %s\n", payload)
		}
		_, err = fmt.Fprintf(w, "收到更新: %s", payload)
		if err != nil {
			return
		}
	})

	// 处理 SSE 连接
	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		// 添加 CORS 头
		// 允许你的前端地址访问
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// SSE 必要的头
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// 如果是预检请求 (OPTIONS)，直接返回 OK
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 调用 sseServer 的处理逻辑
		sseServer.ServeHTTP(w, r)
	})

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
