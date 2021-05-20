// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: LicenseRef-ONF-Member-1.0

package service

import (
	"context"
	"github.com/onosproject/onos-lib-go/pkg/errors"

	"github.com/onosproject/onos-api/go/onos/uenib"
	"github.com/onosproject/onos-lib-go/pkg/logging"
	"github.com/onosproject/onos-lib-go/pkg/northbound"
	"github.com/onosproject/onos-uenib/pkg/store"
	"google.golang.org/grpc"
)

var log = logging.GetLogger("northbound")

// NewService returns a new UE-NIB Service
func NewService(store store.Store) northbound.Service {
	return &Service{
		store: store,
	}
}

// Service is a Service implementation for administration.
type Service struct {
	store store.Store
}

// Register registers the Service with the gRPC server.
func (s Service) Register(r *grpc.Server) {
	server := &Server{
		ues: s.store,
	}
	uenib.RegisterUEServiceServer(r, server)
}

// Server implements the gRPC service for administrative facilities.
type Server struct {
	ues store.Store
}

// Create a new UE entity and its initial set of aspects.
func (s *Server) Create(ctx context.Context, req *uenib.CreateRequest) (*uenib.CreateResponse, error) {
	log.Infof("Received CreateRequest %+v", req)
	ue := req.UE
	err := s.ues.Create(ctx, &ue)
	if err != nil {
		log.Warnf("CreateRequest %+v failed: %v", req, err)
		return nil, errors.Status(err).Err()
	}
	res := &uenib.CreateResponse{UE: ue}
	log.Infof("Sending CreateResponse %+v", res)
	return res, nil
}

// Get a UE entity populated with the requested aspects.
func (s *Server) Get(ctx context.Context, req *uenib.GetRequest) (*uenib.GetResponse, error) {
	log.Infof("Received GetRequest %+v", req)
	ue, err := s.ues.Get(ctx, req.ID, req.AspectTypes...)
	if err != nil {
		log.Warnf("GetRequest %+v failed: %v", req, err)
		return nil, errors.Status(err).Err()
	}
	res := &uenib.GetResponse{UE: *ue}
	log.Infof("Sending GetResponse %+v", res)
	return res, nil
}

// Update an existing UE entity populated with the requested aspects.
func (s *Server) Update(ctx context.Context, req *uenib.UpdateRequest) (*uenib.UpdateResponse, error) {
	log.Infof("Received UpdateRequest %+v", req)
	err := s.ues.Update(ctx, &req.UE)
	if err != nil {
		log.Warnf("UpdateRequest %+v failed: %v", req, err)
		return nil, errors.Status(err).Err()
	}
	res := &uenib.UpdateResponse{UE: req.UE}
	log.Infof("Sending UpdateResponse %+v", res)
	return res, nil
}

// Delete the specified aspects of a UE entity.
func (s *Server) Delete(ctx context.Context, req *uenib.DeleteRequest) (*uenib.DeleteResponse, error) {
	log.Infof("Received DeleteRequest %+v", req)
	err := s.ues.Delete(ctx, req.ID, req.AspectTypes...)
	if err != nil {
		log.Warnf("DeleteRequest %+v failed: %v", req, err)
		return nil, errors.Status(err).Err()
	}
	res := &uenib.DeleteResponse{}
	log.Infof("Sending DeleteResponse %+v", res)
	return res, nil
}

// List returns a stream of UE entities populated the requested aspects.
func (s *Server) List(req *uenib.ListRequest, server uenib.UEService_ListServer) error {
	log.Infof("Received ListRequest %+v", req)
	ch := make(chan *uenib.UE)
	err := s.ues.List(server.Context(), req.AspectTypes, ch, req.Filters)
	if err != nil {
		log.Warnf("ListRequest %+v failed: %v", req, err)
		return errors.Status(err).Err()
	}

	log.Infof("Sending ListResponses")
	for ue := range ch {
		err := server.Send(&uenib.ListResponse{UE: *ue})
		if err != nil {
			return errors.Status(err).Err()
		}
	}
	return nil
}

// Watch returns a stream of UE change notifications, with each UE populated with only the requested aspects.
func (s *Server) Watch(req *uenib.WatchRequest, server uenib.UEService_WatchServer) error {
	log.Infof("Received WatchRequest %+v", req)
	var watchOpts []store.WatchOption
	if !req.Noreplay {
		watchOpts = append(watchOpts, store.WithReplay())
	}

	ch := make(chan uenib.Event)
	if err := s.ues.Watch(server.Context(), req.AspectTypes, ch, req.Filters, watchOpts...); err != nil {
		log.Warnf("WatchTerminationsRequest %+v failed: %v", req, err)
		return errors.Status(err).Err()
	}

	return s.Stream(server, ch)
}

// Stream is the ongoing stream for WatchTerminations request
func (s *Server) Stream(server uenib.UEService_WatchServer, ch chan uenib.Event) error {
	for event := range ch {
		res := &uenib.WatchResponse{Event: event}

		log.Infof("Sending WatchResponse %+v", res)
		if err := server.Send(res); err != nil {
			log.Warnf("WatchResponse %+v failed: %v", res, err)
			return err
		}
	}
	return nil
}
