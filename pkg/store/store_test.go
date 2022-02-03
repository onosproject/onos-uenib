// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"github.com/atomix/atomix-go-client/pkg/atomix/test"
	"github.com/atomix/atomix-go-client/pkg/atomix/test/rsm"
	"github.com/gogo/protobuf/types"
	topoapi "github.com/onosproject/onos-api/go/onos/topo"
	"github.com/onosproject/onos-api/go/onos/uenib"
	"github.com/onosproject/onos-lib-go/pkg/errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestTopoStore(t *testing.T) {
	test := test.NewTest(
		rsm.NewProtocol(),
		test.WithReplicas(1),
		test.WithPartitions(1))
	assert.NoError(t, test.Start())
	defer test.Stop()

	client1, err := test.NewClient("node-1")
	assert.NoError(t, err)

	client2, err := test.NewClient("node-2")
	assert.NoError(t, err)

	store1, err := NewAtomixStore(client1)
	assert.NoError(t, err)
	defer store1.Close()

	store2, err := NewAtomixStore(client2)
	assert.NoError(t, err)
	defer store2.Close()

	aspectTypes := []string{"onos.topo.Location", "onos.uenib.CellInfo"}

	elch := make(chan<- *uenib.UE)
	err = store1.List(context.TODO(), aspectTypes, elch)
	assert.NoError(t, err)

	ch := make(chan uenib.Event)
	err = store2.Watch(context.Background(), aspectTypes, ch)
	assert.NoError(t, err)

	ue1 := &uenib.UE{
		ID:      "u1",
		Aspects: map[string]*types.Any{"onos.topo.Location": {TypeUrl: "onos.topo.Location", Value: []byte(`{"lat": 1.23, "lng": 3.21}`)}},
	}
	err = store1.Create(context.TODO(), ue1)
	assert.NoError(t, err)

	ue2 := &uenib.UE{
		ID:      "u2",
		Aspects: map[string]*types.Any{"onos.topo.Location": {TypeUrl: "onos.topo.Location", Value: []byte(`{"lat": 3.14, "lng": 6.28}`)}},
	}
	err = store1.Create(context.TODO(), ue2)
	assert.NoError(t, err)

	// Verify events were received for the objects
	_, _ = nextEvent(t, ch, "u1", uenib.EventType_ADDED)
	_, _ = nextEvent(t, ch, "u2", uenib.EventType_ADDED)

	// Get the object with existing aspects
	ue, err := store2.Get(context.TODO(), "u1", "onos.topo.Location")
	assert.NoError(t, err)
	assert.NotNil(t, ue)
	assert.Equal(t, uenib.ID("u1"), ue.ID)
	assert.NotNil(t, ue.Aspects["onos.topo.Location"])
	assert.Nil(t, ue.Aspects["onos.uenib.CellInfo"])

	// Get the object with non-existent aspect
	_, err = store2.Get(context.TODO(), "u1", "onos.topo.Location", "onos.uenib.CellInfo")
	assert.Error(t, err)

	// Create another object
	ue3 := &uenib.UE{
		ID:      "u3",
		Aspects: map[string]*types.Any{"onos.uenib.CellInfo": {TypeUrl: "onos.uenib.CellInfo", Value: []byte(`{"serving_cell": {"id": "foo", "signal_strength": 42.0}}`)}},
	}

	err = store2.Create(context.TODO(), ue3)
	assert.NoError(t, err)

	// Verify event was received
	_, _ = nextEvent(t, ch, "u3", uenib.EventType_ADDED)

	// Update one of the objects
	err = ue2.SetAspect(&topoapi.Location{Lat: 1, Lng: 2})
	assert.NoError(t, err)
	err = ue2.SetAspect(&uenib.CellInfo{ServingCell: &uenib.CellConnection{ID: "foo", SignalStrength: 69.0}})
	assert.NoError(t, err)
	err = store1.Update(context.TODO(), ue2)
	assert.NoError(t, err)

	_, et1 := nextEvent(t, ch, "u2", uenib.EventType_NONE) // location was updated
	_, et2 := nextEvent(t, ch, "u2", uenib.EventType_NONE) // cell info was added
	assert.True(t, et1 == uenib.EventType_UPDATED && et2 == uenib.EventType_ADDED || et2 == uenib.EventType_UPDATED && et1 == uenib.EventType_ADDED)

	// List the objects
	lch := make(chan *uenib.UE)
	err = store1.List(context.TODO(), []string{"onos.topo.Location", "onos.uenib.CellInfo"}, lch)
	assert.NoError(t, err)

	c := 0
	for ue := range lch {
		assert.True(t, ue.ID == "u1" || ue.ID == "u2" || ue.ID == "u3")
		c = c + 1
	}
	assert.Equal(t, 3, c)

	// Delete an object
	err = store1.Delete(context.TODO(), ue2.ID, "onos.topo.Location")
	assert.NoError(t, err)

	_, _ = nextEvent(t, ch, "u2", uenib.EventType_REMOVED)

	ue2, err = store2.Get(context.TODO(), "u2", "onos.topo.Location")
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
	assert.Nil(t, ue2)

}

func nextEvent(t *testing.T, ch chan uenib.Event, id uenib.ID, eventType uenib.EventType) (*uenib.UE, uenib.EventType) {
	select {
	case c := <-ch:
		if eventType != uenib.EventType_NONE {
			assert.Equal(t, eventType, c.Type)
		}
		if len(id) > 0 {
			assert.Equal(t, id, c.UE.ID)
		}
		return &c.UE, c.Type
	case <-time.After(5 * time.Second):
		t.FailNow()
	}
	return nil, uenib.EventType_NONE
}
