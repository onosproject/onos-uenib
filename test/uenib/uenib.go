// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package uenib

import (
	"context"
	"github.com/gogo/protobuf/types"
	"github.com/onosproject/onos-api/go/onos/uenib"
	"github.com/onosproject/onos-uenib/test/utils"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// lookupUE uses a Get() operation to query the given UE
func lookupUE(aspectName string, ueID uenib.ID, uenibClient uenib.UEServiceClient) (*uenib.GetUEResponse, error) {
	aspectTypes := []string{aspectName}
	getRequest := &uenib.GetUERequest{
		ID:          ueID,
		AspectTypes: aspectTypes,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	getResponse, err := uenibClient.GetUE(ctx, getRequest)

	cancel()
	return getResponse, err
}

// checkUE makes sure that the contents of the give UE response are correct
func checkUE(t *testing.T, getResponse *uenib.GetUEResponse, ueID uenib.ID, aspectName string, aspectValue string) {
	assert.NotNil(t, getResponse)
	assert.Equal(t, ueID, getResponse.GetUE().ID)
	assert.Equal(t, aspectValue, string(getResponse.UE.Aspects[aspectName].Value))
	assert.Equal(t, aspectName, getResponse.UE.Aspects[aspectName].TypeUrl)
}

// TestUeNib :
func (s *TestSuite) TestUeNib(t *testing.T) {
	const (
		Ue1Id        = uenib.ID("UE1")
		aspect1Name  = "a1"
		aspect2Name  = "a2"
		aspect1Value = "v1"
		aspect2Value = "v2"
	)

	// Create a gRPC client to the onos-uenib component
	uenibClient := utils.GetUeNibClient(t)
	assert.NotNil(t, uenibClient)

	// Make sure that querying a non-existent UE fails
	getNoUEResponse, err := lookupUE(aspect1Name, Ue1Id, uenibClient)
	assert.Error(t, err)
	assert.Nil(t, getNoUEResponse)

	// Create a new UE
	aspects := map[string]*types.Any{}
	v1 := &types.Any{
		Value:   []byte(aspect1Value),
		TypeUrl: aspect1Name,
	}
	v2 := &types.Any{
		Value:   []byte(aspect2Value),
		TypeUrl: aspect2Name,
	}

	aspects[aspect1Name] = v1
	aspects[aspect2Name] = v2

	createRequest := &uenib.CreateUERequest{
		UE: uenib.UE{
			ID:      Ue1Id,
			Aspects: aspects,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	createResponse, err := uenibClient.CreateUE(ctx, createRequest)
	assert.NoError(t, err)
	assert.NotNil(t, createResponse)
	cancel()

	// Query the newly created UE
	getResponse, err := lookupUE(aspect1Name, Ue1Id, uenibClient)
	assert.NoError(t, err)
	assert.NotNil(t, getResponse)
	checkUE(t, getResponse, Ue1Id, aspect1Name, aspect1Value)

	// Update the UE
	v1Updated := &types.Any{
		Value:   []byte("updated"),
		TypeUrl: aspect1Name,
	}
	aspects[aspect1Name] = v1Updated
	updateRequest := &uenib.UpdateUERequest{
		UE: uenib.UE{
			ID:      Ue1Id,
			Aspects: aspects,
		},
	}
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	updateResponse, err := uenibClient.UpdateUE(ctx, updateRequest)
	assert.NoError(t, err)
	assert.NotNil(t, updateResponse)
	cancel()

	// Check that the update happened
	getAfterUpdateResponse, err := lookupUE(aspect1Name, Ue1Id, uenibClient)
	assert.NoError(t, err)
	assert.NotNil(t, getAfterUpdateResponse)
	checkUE(t, getAfterUpdateResponse, Ue1Id, aspect1Name, "updated")

	// Delete the UE
	aspectTypes := []string{aspect1Name}
	deleteRequest := &uenib.DeleteUERequest{
		ID:          Ue1Id,
		AspectTypes: aspectTypes,
	}
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	deleteResponse, err := uenibClient.DeleteUE(ctx, deleteRequest)
	assert.NoError(t, err)
	assert.NotNil(t, deleteResponse)
	cancel()

	// Make sure the delete removed the UE
	getResponseAfterDelete, err := lookupUE(aspect1Name, Ue1Id, uenibClient)
	assert.Nil(t, getResponseAfterDelete)
	assert.Error(t, err)
}
