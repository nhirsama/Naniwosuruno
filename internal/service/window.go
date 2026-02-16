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
		for id, state := range s.clients {
			if state.IsOnline && now.Sub(state.LastHeartbeat) > 360*time.Second {
				state.IsOnline = false
				log.Printf("Client %s (%s) offline (timeout)", state.Name, id)
				s.publishStatusEvent(state.Name, state.OS, "offline")
			}
		}
		s.mu.Unlock()
	}
}

func (s *WindowService) publishStatusEvent(clientName, os, status string) {
	event := &naniwosurunov1.WindowEvent{
		Client: clientName,
		Os:     os,
		Status: status,
	}
	payload, err := json.Marshal(event)
	if err == nil {
		s.sseServer.TryPublish("focus", &sse.Event{Data: payload})
	}
}

func (s *WindowService) ReportWindow(ctx context.Context, req *connect.Request[naniwosurunov1.ReportWindowRequest]) (*connect.Response[naniwosurunov1.ReportWindowResponse], error) {
	// 1. Token Validation (from Header)
	token := req.Header().Get("Authorization")
	// Handle "Bearer <token>"
	if strings.HasPrefix(token, "Bearer ") {
		token = strings.TrimPrefix(token, "Bearer ")
	}
	// Fallback
	if token == "" {
		token = req.Header().Get("token")
	}

	session, ok := s.authenticator.ValidateSession(token)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired token"))
	}

	// Update client state
	s.mu.Lock()
	state, exists := s.clients[session.ClientID]
	if !exists {
		state = &ClientState{Name: session.Name, OS: req.Msg.Os}
		s.clients[session.ClientID] = state
	}
	state.LastHeartbeat = time.Now()
	state.OS = req.Msg.Os
	if !state.IsOnline {
		state.IsOnline = true
		log.Printf("Client %s (%s) online via ReportWindow", session.Name, session.ClientID)
		s.publishStatusEvent(session.Name, state.OS, "online")
	}
	s.mu.Unlock()

	// 2. Publish to SSE (Legacy support for Web Frontend)
	// Use generated Proto struct for type-safe serialization
	event := &naniwosurunov1.WindowEvent{
		Title:  req.Msg.Title,
		Os:     req.Msg.Os,
		Client: session.Name,
		Status: "update",
	}

	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal SSE payload: %v", err)
	} else {
		s.sseServer.TryPublish("focus", &sse.Event{Data: payload})
	}

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

	// 序列号检查
	if req.Msg.Count <= state.LastCount && state.LastCount != 0 {
		s.mu.Unlock()
		return connect.NewResponse(&naniwosurunov1.HeartbeatResponse{
			Count: state.LastCount,
		}), nil
	}

	state.LastCount = req.Msg.Count
	state.LastHeartbeat = time.Now()
	if !state.IsOnline {
		state.IsOnline = true
		log.Printf("Client %s (%s) online via Heartbeat", session.Name, session.ClientID)
		s.publishStatusEvent(session.Name, state.OS, "online")
	}
	s.mu.Unlock()

	return connect.NewResponse(&naniwosurunov1.HeartbeatResponse{
		Count: req.Msg.Count,
	}), nil
}

func (s *WindowService) SubscribeEvents(ctx context.Context, req *connect.Request[naniwosurunov1.SubscribeEventsRequest], stream *connect.ServerStream[naniwosurunov1.WindowEvent]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("RPC streaming not yet implemented"))
}
