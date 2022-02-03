// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/atomix/atomix-go-client/pkg/atomix"
	_map "github.com/atomix/atomix-go-client/pkg/atomix/map"
	"github.com/gogo/protobuf/types"
	"github.com/onosproject/onos-api/go/onos/uenib"
	"github.com/onosproject/onos-lib-go/pkg/errors"
	"github.com/onosproject/onos-lib-go/pkg/logging"
)

var log = logging.GetLogger("store")

// NewAtomixStore returns a new persistent Store
func NewAtomixStore(client atomix.Client) (Store, error) {
	ueAspects, err := client.GetMap(context.Background(), "onos-uenib-objects")
	if err != nil {
		return nil, err
	}

	store := &atomixStore{
		ueAspects:       ueAspects,
		idToAspects:     make(map[uenib.ID]map[string]bool),
		idToAspectsLock: sync.RWMutex{},
	}

	// watch the atomixStore for changes
	// atomixStore consits of a map of ueIDs/ueAspects as keys and values containing the aspect values
	// therefore, we only need to watch the add, remove, and replay to keep proper records of
	// which aspects are present in the map for each ueID
	mapCh := make(chan _map.Event)
	if err := ueAspects.Watch(context.Background(), mapCh, make([]_map.WatchOption, 0)...); err != nil {
		// log.Errorf("Failed to start indexer: %s", err)
		return nil, errors.FromAtomix(err)
	}
	go func() {
		for event := range mapCh {
			id, any := decodeAspect(event.Entry)
			switch event.Type {
			case _map.EventReplay:
				store.registerAspect(id, any.TypeUrl)
			case _map.EventInsert:
				store.registerAspect(id, any.TypeUrl)
			case _map.EventRemove:
				store.unregisterAspect(id, any.TypeUrl)
			}
		}
	}()
	return store, nil
}

// Store stores UE information
type Store interface {
	io.Closer

	// Create a new UE entity and its initial set of aspects.
	Create(ctx context.Context, object *uenib.UE) error

	// Get a UE entity populated with the requested aspects.
	Get(ctx context.Context, id uenib.ID, aspectTypes ...string) (*uenib.UE, error)

	// Update an existing UE entity with the specified aspects.
	Update(ctx context.Context, object *uenib.UE) error

	// Delete specified aspects of a UE entity or delete the UE entity entirely.
	Delete(ctx context.Context, id uenib.ID, aspectTypes ...string) error

	// List stream entities populated the requested aspects on the given channel.
	List(ctx context.Context, aspectTypes []string, ch chan<- *uenib.UE) error

	// Watch streams UE change notifications, with each UE populated with only the changed aspect.
	// Only changes to the requested aspects will be forwarded.
	Watch(ctx context.Context, aspectTypes []string, ch chan<- uenib.Event, opts ...WatchOption) error
}

// WatchOption is a configuration option for Watch calls
type WatchOption interface {
	apply([]_map.WatchOption) []_map.WatchOption
}

// watchReplyOption is an option to replay events on watch
type watchReplayOption struct {
}

func (o watchReplayOption) apply(opts []_map.WatchOption) []_map.WatchOption {
	return append(opts, _map.WithReplay())
}

// WithReplay returns a WatchOption that replays past changes
func WithReplay() WatchOption {
	return watchReplayOption{}
}

// atomixStore is the object implementation of the Store
type atomixStore struct {
	ueAspects       _map.Map
	idToAspects     map[uenib.ID]map[string]bool
	idToAspectsLock sync.RWMutex
}

func aspectKey(id uenib.ID, aspectType string) string {
	return fmt.Sprintf("%s/%s", id, aspectType)
}

func (s *atomixStore) Create(ctx context.Context, ue *uenib.UE) error {
	if ue.ID == "" {
		return errors.NewInvalid("ID cannot be empty")
	}

	for _, aspect := range ue.Aspects {
		key := aspectKey(ue.ID, aspect.TypeUrl)
		log.Infof("Creating UE aspect %s", key)

		_, err := s.ueAspects.Put(ctx, key, aspect.Value, _map.IfNotSet())
		if err != nil {
			err = errors.FromAtomix(err)
			if !errors.IsCanceled(err) && !errors.IsConflict(err) {
				log.Errorf("Failed to create UE aspect %s: %s", key, err)
			}
			return err
		}
	}

	return nil
}

func (s *atomixStore) Update(ctx context.Context, ue *uenib.UE) error {
	if ue.ID == "" {
		return errors.NewInvalid("ID cannot be empty")
	}

	for _, aspect := range ue.Aspects {
		key := aspectKey(ue.ID, aspect.TypeUrl)
		log.Infof("Updating UE aspect %s", key)

		_, err := s.ueAspects.Put(ctx, key, aspect.Value)
		if err != nil {
			err = errors.FromAtomix(err)
			if !errors.IsCanceled(err) && !errors.IsConflict(err) {
				log.Errorf("Failed to update UE aspect %s: %v", key, err)
			}
			return err
		}
	}

	return nil
}

func (s *atomixStore) Get(ctx context.Context, id uenib.ID, aspectTypes ...string) (*uenib.UE, error) {
	if id == "" {
		return nil, errors.NewInvalid("ID cannot be empty")
	}

	ue := &uenib.UE{ID: id, Aspects: map[string]*types.Any{}}

	// If aspect types has 0 length, then use store to get all aspects
	if len(aspectTypes) == 0 {
		s.idToAspectsLock.RLock()
		aspects, ok := s.idToAspects[id]
		if ok {
			for k := range aspects {
				aspectTypes = append(aspectTypes, k)
			}
		}
		s.idToAspectsLock.RUnlock()
	}
	for _, aspectType := range aspectTypes {
		entry, err := s.ueAspects.Get(ctx, aspectKey(ue.ID, aspectType))
		if err != nil {
			return nil, errors.FromAtomix(err)
		}
		ue.Aspects[aspectType] = &types.Any{TypeUrl: aspectType, Value: entry.Value}
	}

	return ue, nil
}

func (s *atomixStore) Delete(ctx context.Context, id uenib.ID, aspectTypes ...string) error {
	if id == "" {
		return errors.NewInvalid("ID cannot be empty")
	}

	if len(aspectTypes) == 0 {
		s.idToAspectsLock.RLock()
		aspects, ok := s.idToAspects[id]
		if ok {
			for k := range aspects {
				aspectTypes = append(aspectTypes, k)
			}
		}
		s.idToAspectsLock.RUnlock()
	}
	for _, aspectType := range aspectTypes {
		key := aspectKey(id, aspectType)
		log.Infof("Deleting UE aspect %s", key)
		_, err := s.ueAspects.Remove(ctx, key)
		if err != nil {
			log.Errorf("Failed to delete UE aspect %s: %s", key, err)
			return errors.FromAtomix(err)
		}
	}
	return nil
}

func typesMap(aspectTypes []string) map[string]string {
	tm := map[string]string{}
	for _, t := range aspectTypes {
		tm[t] = t
	}
	return tm
}

func (s *atomixStore) List(ctx context.Context, aspectTypes []string, ch chan<- *uenib.UE) error {
	mapCh := make(chan _map.Entry)
	if err := s.ueAspects.Entries(ctx, mapCh); err != nil {
		return errors.FromAtomix(err)
	}

	go func() {
		// TODO: Consider more efficient implementation
		ues := map[uenib.ID]*uenib.UE{}

		// if there are now types specified, return everything in the store
		addAnyway := len(aspectTypes) == 0

		relevantTypes := typesMap(aspectTypes)
		for entry := range mapCh {
			id, any := decodeAspect(entry)
			if _, ok := relevantTypes[any.TypeUrl]; ok || addAnyway {
				if ue, ok := ues[id]; ok {
					ue.Aspects[any.TypeUrl] = any
				} else {
					ues[id] = &uenib.UE{ID: id, Aspects: map[string]*types.Any{any.TypeUrl: any}}
				}
			}
		}

		for _, ue := range ues {
			ch <- ue
		}
		close(ch)
	}()

	return nil
}

func (s *atomixStore) Watch(ctx context.Context, aspectTypes []string, ch chan<- uenib.Event, opts ...WatchOption) error {
	watchOpts := make([]_map.WatchOption, 0)
	for _, opt := range opts {
		watchOpts = opt.apply(watchOpts)
	}

	mapCh := make(chan _map.Event)
	if err := s.ueAspects.Watch(ctx, mapCh, watchOpts...); err != nil {
		return errors.FromAtomix(err)
	}

	go func() {
		defer close(ch)
		relevantTypes := typesMap(aspectTypes)
		for event := range mapCh {
			id, any := decodeAspect(event.Entry)
			if _, ok := relevantTypes[any.TypeUrl]; ok {
				var eventType uenib.EventType
				switch event.Type {
				case _map.EventReplay:
					eventType = uenib.EventType_NONE
				case _map.EventInsert:
					eventType = uenib.EventType_ADDED
				case _map.EventRemove:
					eventType = uenib.EventType_REMOVED
				case _map.EventUpdate:
					eventType = uenib.EventType_UPDATED
				default:
					eventType = uenib.EventType_UPDATED
				}
				ch <- uenib.Event{
					Type: eventType,
					UE:   uenib.UE{ID: id, Aspects: map[string]*types.Any{any.TypeUrl: any}},
				}
			}
		}
	}()
	return nil
}

func (s *atomixStore) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_ = s.ueAspects.Close(ctx)
	// TODO: Close the go routine
	defer cancel()
	return s.ueAspects.Close(ctx)
}

func decodeAspect(entry _map.Entry) (uenib.ID, *types.Any) {
	key := strings.SplitN(entry.Key, "/", 2)
	return uenib.ID(key[0]), &types.Any{TypeUrl: key[1], Value: entry.Value}
}

func (s *atomixStore) registerAspect(id uenib.ID, aspect string) {
	s.idToAspectsLock.Lock()
	defer s.idToAspectsLock.Unlock()
	if _, ok := s.idToAspects[id]; !ok {
		s.idToAspects[id] = map[string]bool{}
	}
	s.idToAspects[id][aspect] = true
}

func (s *atomixStore) unregisterAspect(id uenib.ID, aspect string) {
	s.idToAspectsLock.Lock()
	defer s.idToAspectsLock.Unlock()
	delete(s.idToAspects[id], aspect)
	if len(s.idToAspects[id]) == 0 {
		delete(s.idToAspects, id)
	}
}
