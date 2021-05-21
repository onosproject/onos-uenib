// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: LicenseRef-ONF-Member-1.0

package store

import (
	"context"
	"fmt"
	"github.com/atomix/go-client/pkg/client/util/net"
	"github.com/gogo/protobuf/types"
	"github.com/onosproject/onos-lib-go/pkg/errors"
	"github.com/onosproject/onos-lib-go/pkg/logging"
	"github.com/onosproject/onos-uenib/pkg/config"
	"io"
	"strings"
	"time"

	_map "github.com/atomix/go-client/pkg/client/map"
	"github.com/atomix/go-client/pkg/client/primitive"
	"github.com/onosproject/onos-api/go/onos/uenib"
	"github.com/onosproject/onos-lib-go/pkg/atomix"
)

var log = logging.GetLogger("store")

// NewAtomixStore returns a new persistent Store
func NewAtomixStore() (Store, error) {
	ricConfig, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	database, err := atomix.GetDatabase(ricConfig.Atomix, ricConfig.Atomix.GetDatabase(atomix.DatabaseTypeConsensus))
	if err != nil {
		return nil, err
	}

	ueAspects, err := database.GetMap(context.Background(), "ueAspects")
	if err != nil {
		return nil, err
	}

	return &atomixStore{
		ueAspects: ueAspects,
	}, nil
}

// NewLocalStore returns a new local object store
func NewLocalStore() (Store, error) {
	_, address := atomix.StartLocalNode()
	return newLocalStore(address)
}

func newLocalStore(address net.Address) (Store, error) {
	session, err := primitive.NewSession(context.TODO(), primitive.Partition{ID: 1, Address: address})
	if err != nil {
		return nil, err
	}

	ueAspects, err := _map.New(context.Background(), primitive.Name{Namespace: "local", Name: "ueAspect"}, []*primitive.Session{session})
	if err != nil {
		return nil, err
	}

	return &atomixStore{
		ueAspects: ueAspects,
	}, nil
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
	ueAspects _map.Map
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
			log.Errorf("Failed to create UE aspect %s: %s", key, err)
			return errors.FromAtomix(err)
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
			log.Errorf("Failed to update UE aspect %s: %s", key, err)
			return errors.FromAtomix(err)
		}
	}

	return nil
}

func (s *atomixStore) Get(ctx context.Context, id uenib.ID, aspectTypes ...string) (*uenib.UE, error) {
	if id == "" {
		return nil, errors.NewInvalid("ID cannot be empty")
	}

	ue := &uenib.UE{ID: id, Aspects: map[string]*types.Any{}}
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
	mapCh := make(chan *_map.Entry)
	if err := s.ueAspects.Entries(ctx, mapCh); err != nil {
		return errors.FromAtomix(err)
	}

	go func() {
		// TODO: Consider more efficient implementation
		ues := map[uenib.ID]*uenib.UE{}

		relevantTypes := typesMap(aspectTypes)
		for entry := range mapCh {
			id, any := decodeAspect(entry)
			if _, ok := relevantTypes[any.TypeUrl]; ok {
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

	mapCh := make(chan *_map.Event)
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
				case _map.EventNone:
					eventType = uenib.EventType_NONE
				case _map.EventInserted:
					eventType = uenib.EventType_ADDED
				case _map.EventRemoved:
					eventType = uenib.EventType_REMOVED
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
	defer cancel()
	return s.ueAspects.Close(ctx)
}

func decodeAspect(entry *_map.Entry) (uenib.ID, *types.Any) {
	key := strings.SplitN(entry.Key, "/", 2)
	return uenib.ID(key[0]), &types.Any{TypeUrl: key[1], Value: entry.Value}
}
