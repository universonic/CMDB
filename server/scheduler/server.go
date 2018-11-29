// Copyright © 2018 Alfred Chou <unioverlord@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scheduler

import (
	"time"

	cron "github.com/robfig/cron"
	genericStorage "github.com/universonic/cmdb/shared/storage/generic"
	zap "go.uber.org/zap"
)

// Server is the scheduler server, but it does not listen on any port.
// It watches on critical resources and wait for a change, and triggers various operations
// which depends the kind of resource if changes are detected. It is indeed a bundle of
// multiple watchers and callback functions.
type Server struct {
	closeCh          chan struct{}
	clzSubCh         []chan struct{}
	sche             cron.Schedule
	machineObserver  genericStorage.Watcher
	digestObserver   genericStorage.Watcher
	autoDiscObserver genericStorage.Watcher
	storage          genericStorage.Storage
	Handler          *Handler
}

// Prepare initialize inner storage and logger for server
func (in *Server) Prepare(storage genericStorage.Storage, logger *zap.SugaredLogger) {
	in.storage = storage
	in.Handler = NewHandler(storage, logger)
}

// Subscribe attach an external channel to be used for callback function when server exited.
func (in *Server) Subscribe() <-chan struct{} {
	subscription := make(chan struct{}, 1)
	in.clzSubCh = append(in.clzSubCh, subscription)
	return subscription
}

// Serve start the scheduler server.
func (in *Server) Serve() (err error) {
	defer func() {
		for i := range in.clzSubCh {
			close(in.clzSubCh[i])
		}
		in.clzSubCh = in.clzSubCh[:0]
	}()
	in.machineObserver, err = in.storage.Watch(genericStorage.NewMachine(), genericStorage.WatchOnKind)
	if err != nil {
		return
	}
	in.digestObserver, err = in.storage.Watch(genericStorage.NewMachineDigest(), genericStorage.WatchOnKind)
	if err != nil {
		return
	}
	in.autoDiscObserver, err = in.storage.Watch(genericStorage.NewDiscoveredMachines(), genericStorage.WatchOnName)
	if err != nil {
		return
	}
	machineEventChan := in.machineObserver.Output()
	digestEventChan := in.digestObserver.Output()
	autoDiscEventChan := in.autoDiscObserver.Output()
	var (
		revalidate bool
		timer      *time.Timer
	)
	if sche := in.sche.Next(time.Now()); sche.IsZero() {
		revalidate = true
		timer = time.NewTimer(time.Minute)
	} else {
		timer = time.NewTimer(sche.Sub(time.Now()))
	}
LOOP:
	for {
		select {
		case <-in.closeCh:
			break LOOP
		case event := <-machineEventChan:
			switch event.Type {
			case genericStorage.DELETE:
				// go in.Handler.gcMachineOnEvent(event)
			case genericStorage.ERROR:
				break LOOP
			}
		case event := <-digestEventChan:
			switch event.Type {
			case genericStorage.CREATE:
				go in.Handler.refreshMachineSnapshotOnEvent(event)
			case genericStorage.ERROR:
				break LOOP
			}
		case event := <-autoDiscEventChan:
			switch event.Type {
			case genericStorage.CREATE, genericStorage.UPDATE:
				go in.Handler.refreshDicoveredMachinesOnEvent(event)
			}
		case <-timer.C:
			if sche := in.sche.Next(time.Now()); sche.IsZero() {
				revalidate = true
				timer.Reset(time.Minute)
			} else {
				revalidate = false
				timer.Reset(sche.Sub(time.Now()))
			}
			if !revalidate {
				go in.Handler.createMachineDigestOnTime()
				go in.Handler.autoDiscoveryOnTime()
			}
		}
	}

	timer.Stop()
	in.machineObserver.Close()
	in.digestObserver.Close()
	return nil
}

// Stop shutdown the server gracefully if possible.
func (in *Server) Stop() {
	close(in.closeCh)
}

// NewServer returns an empty scheduler server
func NewServer(exp string) (*Server, error) {
	sche, err := cron.Parse(exp)
	if err != nil {
		return nil, err
	}
	return &Server{
		closeCh: make(chan struct{}, 1),
		sche:    sche,
	}, nil
}
