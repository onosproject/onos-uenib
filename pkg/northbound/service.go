// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

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

// CreateUE creates a new UE entity and its initial set of aspects.
func (s *Server) CreateUE(ctx context.Context, req *uenib.CreateUERequest) (*uenib.CreateUEResponse, error) {
	log.Infof("Received CreateUERequest %+v", req)
	err := s.ues.Create(ctx, &req.UE)
	if err != nil {
		log.Warnf("CreateUERequest %+v failed: %v", req, err)
		return nil, errors.Status(err).Err()
	}
	res := &uenib.CreateUEResponse{}
	log.Infof("Sending CreateUEResponse %+v", res)
	return res, nil
}

// GetUE retrieves a UE entity populated with the requested aspects.
func (s *Server) GetUE(ctx context.Context, req *uenib.GetUERequest) (*uenib.GetUEResponse, error) {
	log.Infof("Received GetUERequest %+v", req)
	ue, err := s.ues.Get(ctx, req.ID, req.AspectTypes...)
	if err != nil {
		log.Warnf("GetUERequest %+v failed: %v", req, err)
		return nil, errors.Status(err).Err()
	}
	res := &uenib.GetUEResponse{UE: *ue}
	log.Infof("Sending GetUEResponse %+v", res)
	return res, nil
}

// UpdateUE updated an existing UE entity populated with the requested aspects.
func (s *Server) UpdateUE(ctx context.Context, req *uenib.UpdateUERequest) (*uenib.UpdateUEResponse, error) {
	log.Infof("Received UpdateUERequest %+v", req)
	err := s.ues.Update(ctx, &req.UE)
	if err != nil {
		log.Warnf("UpdateUERequest %+v failed: %v", req, err)
		return nil, errors.Status(err).Err()
	}
	res := &uenib.UpdateUEResponse{}
	log.Infof("Sending UpdateUEResponse %+v", res)
	return res, nil
}

// DeleteUE deletes the specified aspects of a UE entity.
func (s *Server) DeleteUE(ctx context.Context, req *uenib.DeleteUERequest) (*uenib.DeleteUEResponse, error) {
	log.Infof("Received DeleteUERequest %+v", req)
	err := s.ues.Delete(ctx, req.ID, req.AspectTypes...)
	if err != nil {
		log.Warnf("DeleteUERequest %+v failed: %v", req, err)
		return nil, errors.Status(err).Err()
	}
	res := &uenib.DeleteUEResponse{}
	log.Infof("Sending DeleteUEResponse %+v", res)
	return res, nil
}

// ListUEs returns a stream of UE entities populated the requested aspects.
func (s *Server) ListUEs(req *uenib.ListUERequest, server uenib.UEService_ListUEsServer) error {
	log.Infof("Received ListUERequest %+v", req)
	ch := make(chan *uenib.UE)
	err := s.ues.List(server.Context(), req.AspectTypes, ch)
	if err != nil {
		log.Warnf("ListUERequest %+v failed: %v", req, err)
		return errors.Status(err).Err()
	}

	log.Infof("Sending ListUEResponses")
	for ue := range ch {
		err := server.Send(&uenib.ListUEResponse{UE: *ue})
		if err != nil {
			return errors.Status(err).Err()
		}
	}
	return nil
}

// WatchUEs returns a stream of UE change notifications, with each UE populated with only the requested aspects.
func (s *Server) WatchUEs(req *uenib.WatchUERequest, server uenib.UEService_WatchUEsServer) error {
	log.Infof("Received WatchUERequest %+v", req)
	var watchOpts []store.WatchOption
	if !req.Noreplay {
		watchOpts = append(watchOpts, store.WithReplay())
	}

	ch := make(chan uenib.Event)
	if err := s.ues.Watch(server.Context(), req.AspectTypes, ch, watchOpts...); err != nil {
		log.Warnf("WatchTerminationsUERequest %+v failed: %v", req, err)
		return errors.Status(err).Err()
	}

	return s.Stream(server, ch)
}

// Stream is the ongoing stream for WatchTerminations request
func (s *Server) Stream(server uenib.UEService_WatchUEsServer, ch chan uenib.Event) error {
	for event := range ch {
		res := &uenib.WatchUEResponse{Event: event}

		log.Infof("Sending WatchUEResponse %+v", res)
		if err := server.Send(res); err != nil {
			log.Warnf("WatchUEResponse %+v failed: %v", res, err)
			return err
		}
	}
	return nil
}
