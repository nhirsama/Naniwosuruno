package service

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	naniwosurunov1 "github.com/nhirsama/Naniwosuruno/gen/naniwosuruno/v1"
	"github.com/nhirsama/Naniwosuruno/pkg/auth"
)

type AuthService struct {
	authenticator auth.StatefulAuthenticator
}

func NewAuthService(auth auth.StatefulAuthenticator) *AuthService {
	return &AuthService{authenticator: auth}
}

func (s *AuthService) CreateChallenge(ctx context.Context, req *connect.Request[naniwosurunov1.CreateChallengeRequest]) (*connect.Response[naniwosurunov1.CreateChallengeResponse], error) {
	clientID := req.Msg.ClientId
	if clientID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("client_id is required"))
	}

	challenge, err := s.authenticator.CreateChallenge(clientID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&naniwosurunov1.CreateChallengeResponse{
		Challenge: challenge,
	}), nil
}

func (s *AuthService) VerifyChallenge(ctx context.Context, req *connect.Request[naniwosurunov1.VerifyChallengeRequest]) (*connect.Response[naniwosurunov1.VerifyChallengeResponse], error) {
	clientID := req.Msg.ClientId
	signature := req.Msg.Signature

	if clientID == "" || signature == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("client_id and signature are required"))
	}

	token, expiresIn, err := s.authenticator.ValidateChallengeAndIssueToken(clientID, signature)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication failed"))
	}

	return connect.NewResponse(&naniwosurunov1.VerifyChallengeResponse{
		Token:     token,
		ExpiresIn: expiresIn,
	}), nil
}
