package repository

import (
	"encoding/json"
	"fmt"

	"github.com/cloudfoundry/cli/cf/api/authentication"
	"github.com/cloudfoundry/go-ccapi/v3/client"
)

//go:generate counterfeiter -o fakes/fake_token_handler.go . TokenHandler
type TokenHandler interface {
	Do(cb func() ([]byte, error)) ([]byte, error)
}

type tokenHandler struct {
	ccClient       client.Client
	tokenRefresher authentication.TokenRefresher
}

const invalidTokenCode = 1000

func NewTokenHandler(
	ccClient client.Client,
	tokenRefresher authentication.TokenRefresher,
) TokenHandler {
	return &tokenHandler{
		ccClient:       ccClient,
		tokenRefresher: tokenRefresher,
	}
}

func (t *tokenHandler) Do(cb func() ([]byte, error)) ([]byte, error) {
	responseJSON, err := cb()
	if err != nil {
		return responseJSON, err
	}

	response := struct {
		Code int
	}{}
	json.Unmarshal(responseJSON, &response)

	if response.Code == invalidTokenCode {
		token, err := t.tokenRefresher.RefreshAuthToken()
		if err != nil {
			return responseJSON, fmt.Errorf("Failed to refresh auth token: %s", err.Error())
		}

		t.ccClient.SetToken(token)

		return cb()
	}

	return responseJSON, nil
}
