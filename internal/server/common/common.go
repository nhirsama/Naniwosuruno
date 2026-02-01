package common

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/r3labs/sse/v2"
)

func GetTokenFromRequest(r *http.Request) string {
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}
	if c, err := r.Cookie("token"); err == nil {
		return c.Value
	}
	return ""
}

func ServeSSE(sseServer *sse.Server, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	sseServer.ServeHTTP(w, r)
}

// ProcessUpdate 处理具体的窗口信息更新逻辑，并分发到 SSE 流中
func ProcessUpdate(sseServer *sse.Server, w http.ResponseWriter, r *http.Request, clientName string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		log.Printf("Reading body failed: %v", err)
		return
	}

	var updateData struct {
		Title string `json:"title"`
		OS    string `json:"os"`
	}
	_ = json.Unmarshal(body, &updateData)

	// 构造统一格式的消息负载，包含客户端显示名称
	payload := fmt.Sprintf(`{"title": %q, "os": %q, "client": %q}`, updateData.Title, updateData.OS, clientName)
	sseServer.TryPublish("focus", &sse.Event{Data: []byte(payload)})

	fmt.Fprintf(w, "Update received from %s", clientName)
}
