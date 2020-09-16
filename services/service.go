// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package services

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

// Service defines a service interface
type Service interface {
	// Start starts the service
	Start(context.Context) error
	// Stop stops the service
	Stop(context.Context) error
	// ExecutableSchema returns an executable schema
	ExecutableSchema() graphql.ExecutableSchema
}

type serviceClientContextKey struct{}

// WithServiceClient adds service client to context
func WithServiceClient(ctx context.Context, client iotexapi.APIServiceClient) context.Context {
	return context.WithValue(ctx, serviceClientContextKey{}, client)
}

// ServiceClient returns the service client
func ServiceClient(ctx context.Context) (iotexapi.APIServiceClient, bool) {
	client, ok := ctx.Value(serviceClientContextKey{}).(iotexapi.APIServiceClient)
	return client, ok
}
