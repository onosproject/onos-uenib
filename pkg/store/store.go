// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/atomix/go-sdk/pkg/primitive"
	"github.com/atomix/go-sdk/pkg/types"
	"github.com/google/uuid"

	_map "github.com/atomix/go-sdk/pkg/primitive/map"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/onosproject/onos-api/go/onos/uenib"
	"github.com/onosproject/onos-lib-go/pkg/errors"
	"github.com/onosproject/onos-lib-go/pkg/logging"
)

var log = logging.GetLogger()

// NewAtomixStore returns a new persistent Store
func NewAtomixStore(client primitive.Client) (Store, error) {
	ueAspects, err := _map.NewBuilder[uenib.ID, *uenib.UE](client, "onos-uenib-objects").
		Tag("onos-uenib", "objects").
		Codec(types.Proto[*uenib.UE](&uenib.UE{})).
		Get(context.Background())
	if err != nil {
		return nil, errors.FromAtomix(err)
	}

	store := &atomixStore{
		ueAspects: ueAspects,
		//idToAspects:     make(map[uenib.ID]map[string]bool),
		//idToAspectsLock: sync.RWMutex{},
		cache:    make(map[uenib.ID]uenib.UE),
		watchers: make(map[uuid.UUID]chan<- uenib.Event),
	}

	events, err := ueAspects.Events(context.Background())
	if err != nil {
		return nil, errors.FromAtomix(err)
	}
	entries, err := ueAspects.List(context.Background())
	if err != nil {
		return nil, errors.FromAtomix(err)
	}
	go store.watchStoreEvents(entries, events)
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
	apply(*watchOptions)
}

// watchReplyOption is an option to replay events on watch
type watchReplayOption struct {
	replay bool
}

func (o watchReplayOption) apply(opts *watchOptions) {
	opts.replay = o.replay
}

// WithReplay returns a WatchOption that replays past changes
func WithReplay() WatchOption {
	return watchReplayOption{true}
}

type watchOptions struct {
	replay bool
}

// atomixStore is the object implementation of the Store
type atomixStore struct {
	ueAspects _map.Map[uenib.ID, *uenib.UE]
	cache     map[uenib.ID]uenib.UE
	cacheMu   sync.RWMutex
	//idToAspects     map[uenib.ID]map[string]bool
	//idToAspectsLock sync.RWMutex
	watchers   map[uuid.UUID]chan<- uenib.Event
	watchersMu sync.RWMutex
}

//func aspectKey(id uenib.ID, aspectType string) string {
//	return fmt.Sprintf("%s/%s", id, aspectType)
//}

func (s *atomixStore) watchStoreEvents(entries _map.EntryStream[uenib.ID, *uenib.UE], events _map.EventStream[uenib.ID, *uenib.UE]) {
	for {
		entry, err := entries.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorf(err.Error())
			continue
		}

		ue := entry.Value

		s.cacheMu.Lock()
		s.cache[ue.ID] = *ue
		s.cacheMu.Unlock()

		s.watchersMu.RLock()
		for _, watcher := range s.watchers {
			watcher <- uenib.Event{
				Type: uenib.EventType_NONE,
				UE:   *ue,
			}
		}
		s.watchersMu.RUnlock()
	}

	for {
		event, err := events.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorf(err.Error())
			continue
		}

		var eventType uenib.EventType
		var ue *uenib.UE
		switch e := event.(type) {
		case *_map.Inserted[uenib.ID, *uenib.UE]:
			ue = e.Entry.Value
			eventType = uenib.EventType_ADDED
			s.cacheMu.Lock()
			s.cache[ue.ID] = *ue
			s.cacheMu.Unlock()
		case *_map.Updated[uenib.ID, *uenib.UE]:
			ue = e.Entry.Value
			eventType = uenib.EventType_UPDATED
			s.cacheMu.Lock()
			s.cache[ue.ID] = *ue
			s.cacheMu.Unlock()
		case *_map.Removed[uenib.ID, *uenib.UE]:
			ue = e.Entry.Value
			eventType = uenib.EventType_REMOVED
			s.cacheMu.Lock()
			s.cache[ue.ID] = *ue
			s.cacheMu.Unlock()
		}

		s.watchersMu.RLock()
		for _, watcher := range s.watchers {
			watcher <- uenib.Event{
				Type: eventType,
				UE:   *ue,
			}
		}
		s.watchersMu.RUnlock()
	}
}

func (s *atomixStore) Create(ctx context.Context, ue *uenib.UE) error {
	if ue.ID == "" {
		return errors.NewInvalid("ID cannot be empty")
	}

	_, err := s.ueAspects.Insert(ctx, ue.ID, ue)
	if err != nil {
		err = errors.FromAtomix(err)
		if !errors.IsAlreadyExists(err) {
			log.Errorf("Failed to create UE %+v: %+v", ue, err)
		} else {
			log.Warnf("Failed to create UE %+v: %+v", ue, err)
		}
		return err
	}

	return nil
}

func (s *atomixStore) Update(ctx context.Context, ue *uenib.UE) error {
	if ue.ID == "" {
		return errors.NewInvalid("ID cannot be empty")
	}

	_, err := s.ueAspects.Update(ctx, ue.ID, ue)
	if err != nil {
		err = errors.FromAtomix(err)
		if !errors.IsNotFound(err) && !errors.IsConflict(err) {
			log.Errorf("Failed to update sub %+v: %+v", ue, err)
		} else {
			log.Warnf("Failed to update sub %+v: %+v", ue, err)
		}
		return err
	}

	return nil
}

func (s *atomixStore) Get(ctx context.Context, id uenib.ID, aspectTypes ...string) (*uenib.UE, error) {
	if id == "" {
		return nil, errors.NewInvalid("ID cannot be empty")
	}

	entry, err := s.ueAspects.Get(ctx, id)
	if err != nil {
		err = errors.FromAtomix(err)
		if !errors.IsNotFound(err) {
			log.Errorf("Failed to get UE %+v: %+v", id, err)
		} else {
			log.Warnf("Failed to get UE %+v: %+v", id, err)
		}
		return nil, err
	}
	ue := &uenib.UE{ID: id, Aspects: map[string]*gogotypes.Any{}}
	if len(aspectTypes) == 0 {
		return entry.Value, nil
	}

	for _, aspectType := range aspectTypes {
		if _, ok := entry.Value.Aspects[aspectType]; !ok {
			return nil, errors.FromAtomix(fmt.Errorf("there is no aspect type %+v", aspectType))
		}
		ue.Aspects[aspectType] = entry.Value.Aspects[aspectType]
	}
	return ue, nil
}

func hasAspectTypes(aspectType string, aspectTypes []string) bool {
	for _, t := range aspectTypes {
		if aspectType == t {
			return true
		}
	}
	return false
}

func (s *atomixStore) Delete(ctx context.Context, id uenib.ID, aspectTypes ...string) error {
	if id == "" {
		return errors.NewInvalid("ID cannot be empty")
	}

	if len(aspectTypes) == 0 {
		_, err := s.ueAspects.Remove(ctx, id)
		if err != nil {
			err = errors.FromAtomix(err)
			if !errors.IsNotFound(err) && !errors.IsConflict(err) {
				log.Errorf("Failed to delete ue %+v: %+v", id, err)
			} else {
				log.Warnf("Failed to delete ue %+v: %+v", id, err)
			}
			return err
		}
		return nil
	}

	entry, err := s.ueAspects.Get(ctx, id)
	if err != nil {
		err = errors.FromAtomix(err)
		if !errors.IsNotFound(err) {
			log.Errorf("Failed to get UE %+v: %+v", id, err)
		} else {
			log.Warnf("Failed to get UE %+v: %+v", id, err)
		}
		return err
	}

	ue := &uenib.UE{ID: id, Aspects: map[string]*gogotypes.Any{}}
	for k, v := range entry.Value.Aspects {
		if !hasAspectTypes(k, aspectTypes) {
			ue.Aspects[k] = v
		}
	}

	if len(ue.Aspects) == 0 {
		_, err = s.ueAspects.Remove(ctx, id)
		if err != nil {
			err = errors.FromAtomix(err)
			if !errors.IsNotFound(err) && !errors.IsConflict(err) {
				log.Errorf("Failed to delete ue %+v: %+v", id, err)
			} else {
				log.Warnf("Failed to delete ue %+v: %+v", id, err)
			}
			return err
		}
		return nil
	}

	return s.Update(ctx, ue)
}

func (s *atomixStore) List(ctx context.Context, aspectTypes []string, ch chan<- *uenib.UE) error {
	list, err := s.ueAspects.List(ctx)
	if err != nil {
		return errors.FromAtomix(err)
	}

	go func() {
		for {
			entry, err := list.Next()
			if err == io.EOF {
				close(ch)
				return
			}
			if err != nil {
				log.Errorf(err.Error())
				continue
			}
			ue := &uenib.UE{ID: entry.Value.ID, Aspects: map[string]*gogotypes.Any{}}
			for _, aspectType := range aspectTypes {
				if v, ok := entry.Value.Aspects[aspectType]; ok {
					ue.Aspects[aspectType] = v
				} else {
					log.Warnf("aspect type %+v is not defined in UE %+v", aspectType, *entry.Value)
				}
			}

			ch <- entry.Value
		}
	}()

	return nil
}

func (s *atomixStore) Watch(ctx context.Context, aspectTypes []string, ch chan<- uenib.Event, opts ...WatchOption) error {
	var watchOpts watchOptions
	for _, opt := range opts {
		opt.apply(&watchOpts)
	}

	// create separate channel for replay and watch event
	replayCh := make(chan uenib.UE)
	eventCh := make(chan uenib.Event)

	go func() {
		defer close(ch)

	replayLoop:
		// process the replay channel first
		for {
			select {
			case ue, ok := <-replayCh:
				if !ok {
					break replayLoop
				}

				ch <- uenib.Event{
					Type: uenib.EventType_NONE,
					UE:   ue,
				}
			case <-ctx.Done():
				go func() {
					for range replayCh {
					}
				}()
			}
		}
	eventLoop:
		for {
			select {
			case event, ok := <-eventCh:
				if !ok {
					break eventLoop
				}
				ch <- event
			case <-ctx.Done():
				go func() {
					for range eventCh {
					}
				}()
			}
		}
	}()

	watchID := uuid.New()
	s.watchersMu.Lock()
	s.watchers[watchID] = eventCh
	s.watchersMu.Unlock()

	var ues []uenib.UE
	if watchOpts.replay {
		s.cacheMu.RLock()
		ues = make([]uenib.UE, 0, len(s.cache))
		for _, ue := range s.cache {
			ues = append(ues, ue)
		}
		s.cacheMu.RUnlock()
	}

	go func() {
		defer close(replayCh)
		for _, ue := range ues {
			replayCh <- ue
		}
	}()

	go func() {
		<-ctx.Done()
		s.watchersMu.Lock()
		delete(s.watchers, watchID)
		s.watchersMu.Unlock()
		close(eventCh)
	}()
	return nil
}

func (s *atomixStore) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := s.ueAspects.Close(ctx)
	// TODO: Close the go routine
	defer cancel()
	if err != nil {
		return errors.FromAtomix(err)
	}
	return nil
}
