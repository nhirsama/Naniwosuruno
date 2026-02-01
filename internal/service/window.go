package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"

	"connectrpc.com/connect"
	naniwosurunov1 "github.com/nhirsama/Naniwosuruno/gen/naniwosuruno/v1"
	"github.com/nhirsama/Naniwosuruno/pkg/auth"
	"github.com/r3labs/sse/v2"
)

type WindowService struct {
	sseServer     *sse.Server
	authenticator auth.StatefulAuthenticator
}

func NewWindowService(sse *sse.Server, auth auth.StatefulAuthenticator) *WindowService {
	return &WindowService{
		sseServer:     sse,
		authenticator: auth,
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

	// 2. Publish to SSE (Legacy support for Web Frontend)
	// Use generated Proto struct for type-safe serialization
	event := &naniwosurunov1.WindowEvent{
		Title:  req.Msg.Title,
		Os:     req.Msg.Os,
		Client: session.Name,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal SSE payload: %v", err)
	} else {
		s.sseServer.TryPublish("focus", &sse.Event{Data: payload})
	}

	return connect.NewResponse(&naniwosurunov1.ReportWindowResponse{}), nil
}

func (s *WindowService) SubscribeEvents(ctx context.Context, req *connect.Request[naniwosurunov1.SubscribeEventsRequest], stream *connect.ServerStream[naniwosurunov1.WindowEvent]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("RPC streaming not yet implemented"))
}
