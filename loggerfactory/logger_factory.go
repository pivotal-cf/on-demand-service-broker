// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package loggerfactory

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/pborman/uuid"

	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
)

const Flags = log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC

type LoggerFactory struct {
	out  io.Writer
	name string
	flag int
}

func New(out io.Writer, name string, flag int) *LoggerFactory {
	return &LoggerFactory{out: out, name: name, flag: flag}
}

func (l *LoggerFactory) NewWithContext(ctx context.Context) *log.Logger {
	if brokercontext.GetReqID(ctx) == "" {
		return l.New()
	}

	prefix := fmt.Sprintf("[%s] [%s] ", l.name, brokercontext.GetReqID(ctx))
	return log.New(l.out, prefix, l.flag)
}

func (l *LoggerFactory) NewWithRequestID() *log.Logger {
	prefix := fmt.Sprintf("[%s] [%s] ", l.name, uuid.New())
	return log.New(l.out, prefix, l.flag)
}

func (l *LoggerFactory) New() *log.Logger {
	prefix := fmt.Sprintf("[%s] ", l.name)
	return log.New(l.out, prefix, l.flag)
}
