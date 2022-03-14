// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"github.com/onosproject/onos-api/go/onos/uenib"
	"github.com/onosproject/onos-lib-go/pkg/grpc/retry"
	"github.com/onosproject/onos-ric-sdk-go/pkg/e2/creds"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	// UeNibServiceAddress has UENIB endpoint
	UeNibServiceAddress = "onos-uenib:5150"
)

// ConnectUeNibServiceHost connects to UE NIB service
func ConnectUeNibServiceHost() (*grpc.ClientConn, error) {
	tlsConfig, err := creds.GetClientCredentials()
	if err != nil {
		return nil, err
	}
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	}
	opts = append(opts, grpc.WithUnaryInterceptor(retry.RetryingUnaryClientInterceptor()))
	return grpc.DialContext(context.Background(), UeNibServiceAddress, opts...)
}

// GetUeNibClient returns a UENIB service client
func GetUeNibClient(t *testing.T) uenib.UEServiceClient {
	conn, err := ConnectUeNibServiceHost()
	assert.NoError(t, err)
	assert.NotNil(t, conn)
	return uenib.CreateUEServiceClient(conn)
}
