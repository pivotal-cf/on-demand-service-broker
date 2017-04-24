// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package brokercontext

import "context"

type correlationIDType int

const (
	operationKey   correlationIDType = iota
	requestIDKey   correlationIDType = iota
	serviceNameKey correlationIDType = iota
	instanceIDKey  correlationIDType = iota
	boshTaskIDKey  correlationIDType = iota
)

func New(ctx context.Context, operation, requestID, serviceName, instanceID string) context.Context {
	ctx = WithOperation(ctx, operation)
	ctx = WithReqID(ctx, requestID)
	ctx = WithServiceName(ctx, serviceName)
	ctx = WithInstanceID(ctx, instanceID)
	return ctx
}

func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, operationKey, operation)
}

func GetOperation(ctx context.Context) string {
	operation, _ := ctx.Value(operationKey).(string)
	return operation
}

func WithReqID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, requestIDKey, reqID)
}

func GetReqID(ctx context.Context) string {
	reqID, _ := ctx.Value(requestIDKey).(string)
	return reqID
}

func WithServiceName(ctx context.Context, serviceName string) context.Context {
	return context.WithValue(ctx, serviceNameKey, serviceName)
}

func GetServiceName(ctx context.Context) string {
	serviceName, _ := ctx.Value(serviceNameKey).(string)
	return serviceName
}

func WithInstanceID(ctx context.Context, instanceID string) context.Context {
	return context.WithValue(ctx, instanceIDKey, instanceID)
}

func GetInstanceID(ctx context.Context) string {
	instanceID, _ := ctx.Value(instanceIDKey).(string)
	return instanceID
}

func WithBoshTaskID(ctx context.Context, boshTaskID int) context.Context {
	return context.WithValue(ctx, boshTaskIDKey, boshTaskID)
}

func GetBoshTaskID(ctx context.Context) int {
	boshTaskID, _ := ctx.Value(boshTaskIDKey).(int)
	return boshTaskID
}
