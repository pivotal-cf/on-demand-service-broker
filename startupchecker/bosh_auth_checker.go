package startupchecker

import (
	"fmt"
	"log"
)

type BOSHAuthChecker struct {
	authVerifier AuthVerifier
	logger       *log.Logger
}

func NewBOSHAuthChecker(authVerifier AuthVerifier, logger *log.Logger) *BOSHAuthChecker {
	return &BOSHAuthChecker{
		authVerifier: authVerifier,
		logger:       logger,
	}
}

func (c *BOSHAuthChecker) Check() error {
	err := c.authVerifier.VerifyAuth(c.logger)
	if err != nil {
		return fmt.Errorf("BOSH Director error: %s", err.Error())
	}
	return nil
}

//go:generate counterfeiter -o fakes/fake_auth_verifier.go . AuthVerifier
type AuthVerifier interface {
	VerifyAuth(*log.Logger) error
}
