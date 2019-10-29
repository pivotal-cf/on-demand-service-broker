// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_auth_verifier.go . AuthVerifier
type AuthVerifier interface {
	VerifyAuth(*log.Logger) error
}
