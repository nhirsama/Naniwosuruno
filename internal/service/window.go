package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	naniwosurunov1 "github.com/nhirsama/Naniwosuruno/gen/naniwosuruno/v1"
	"github.com/nhirsama/Naniwosuruno/pkg/auth"
	"github.com/r3labs/sse/v2"
)

type ClientState struct {
	LastHeartbeat time.Time
	LastCount     uint32
	IsOnline      bool
	Name          string
	OS            string
	LastTitle     string
}

type WindowService struct {
	sseServer     *sse.Server
	authenticator auth.StatefulAuthenticator
	clients       map[string]*ClientState
	mu            sync.Mutex
}

func NewWindowService(sse *sse.Server, auth auth.StatefulAuthenticator) *WindowService {
	s := &WindowService{
		sseServer:     sse,
		authenticator: auth,
		clients:       make(map[string]*ClientState),
	}
	go s.startTimeoutChecker()
	return s
}

func (s *WindowService) startTimeoutChecker() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for _, state := range s.clients {
			if state.IsOnline && now.Sub(state.LastHeartbeat) > 360*time.Second {
				state.IsOnline = false
				log.Printf("Client %s offline (timeout)", state.Name)
				s.publishEvent(state, "offline")
			}
		}
		s.mu.Unlock()
	}
}

// publishEvent 核心修复：手动构造 Map 确保 JSON 字段完整，防止前端解析 title.trim() 报错
func (s *WindowService) publishEvent(state *ClientState, status string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"title":  state.LastTitle,
		"os":     state.OS,
		"client": state.Name,
		"status": status,
	})
	s.sseServer.TryPublish("focus", &sse.Event{Data: payload})
}

func (s *WindowService) ReportWindow(ctx context.Context, req *connect.Request[naniwosurunov1.ReportWindowRequest]) (*connect.Response[naniwosurunov1.ReportWindowResponse], error) {
	token := req.Header().Get("Authorization")
	if strings.HasPrefix(token, "Bearer ") {
		token = strings.TrimPrefix(token, "Bearer ")
	}
	if token == "" {
		token = req.Header().Get("token")
	}

	session, ok := s.authenticator.ValidateSession(token)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired token"))
	}

	s.mu.Lock()
	state, exists := s.clients[session.ClientID]
	if !exists {
		state = &ClientState{Name: session.Name, OS: req.Msg.Os}
		s.clients[session.ClientID] = state
	}
	state.LastHeartbeat = time.Now()
	state.OS = req.Msg.Os
	state.LastTitle = req.Msg.Title
	state.IsOnline = true
	s.mu.Unlock()

	// 统一作为 online 状态发布，确保 title 字段原样发送
	s.publishEvent(state, "online")

	return connect.NewResponse(&naniwosurunov1.ReportWindowResponse{}), nil
}

func (s *WindowService) Heartbeat(ctx context.Context, req *connect.Request[naniwosurunov1.HeartbeatRequest]) (*connect.Response[naniwosurunov1.HeartbeatResponse], error) {
	token := req.Header().Get("Authorization")
	if strings.HasPrefix(token, "Bearer ") {
		token = strings.TrimPrefix(token, "Bearer ")
	}
	if token == "" {
		token = req.Header().Get("token")
	}

	session, ok := s.authenticator.ValidateSession(token)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired token"))
	}

	s.mu.Lock()
	state, exists := s.clients[session.ClientID]
	if !exists {
		state = &ClientState{Name: session.Name}
		s.clients[session.ClientID] = state
	}

	if req.Msg.Count <= state.LastCount && state.LastCount != 0 {
		s.mu.Unlock()
		return connect.NewResponse(&naniwosurunov1.HeartbeatResponse{Count: state.LastCount}), nil
	}

	state.LastCount = req.Msg.Count
	state.LastHeartbeat = time.Now()

	if !state.IsOnline {
		state.IsOnline = true
		log.Printf("Client %s online via Heartbeat", session.Name)
		s.publishEvent(state, "online")
	}
	s.mu.Unlock()

	return connect.NewResponse(&naniwosurunov1.HeartbeatResponse{Count: req.Msg.Count}), nil
}

func (s *WindowService) SubscribeEvents(ctx context.Context, req *connect.Request[naniwosurunov1.SubscribeEventsRequest], stream *connect.ServerStream[naniwosurunov1.WindowEvent]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("RPC streaming not yet implemented"))
}
