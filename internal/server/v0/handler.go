package v0

import (
	"net/http"

	"github.com/nhirsama/Naniwosuruno/internal/server/common"
	"github.com/nhirsama/Naniwosuruno/pkg"
	"github.com/r3labs/sse/v2"
)

type Handler struct {
	ConfigManager *pkg.ConfigManager
	SSEServer     *sse.Server
}

func NewHandler(cm *pkg.ConfigManager, sse *sse.Server) *Handler {
	return &Handler{
		ConfigManager: cm,
		SSEServer:     sse,
	}
}

func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.validateToken(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	common.ProcessUpdate(h.SSEServer, w, r, "Legacy Client")
}

func (h *Handler) HandleEvents(w http.ResponseWriter, r *http.Request) {
	common.ServeSSE(h.SSEServer, w, r)
}

func (h *Handler) validateToken(r *http.Request) bool {
	token := common.GetTokenFromRequest(r)
	if token == "" {
		return false
	}

	cfg := h.ConfigManager.GetConfig()
	return cfg.Token != "" && token == cfg.Token
}
