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

package generic

import (
	"encoding/json"
	"time"
)

// Object indicates a generic type of data object
type Object interface {
	SortableObject
	HasNamespace() bool
}

// SortableObject is a generic sortable object type
type SortableObject interface {
	SetGUID(id string)
	GetGUID() string
	SetKind(kind string)
	GetKind() string
	SetName(name string)
	GetName() string
	SetNamespace(ns string)
	GetNamespace() string
	SetCreationTimestamp(timestamp time.Time)
	GetCreationTimestamp() time.Time
	SetUpdatingTimestamp(timestamp time.Time)
	GetUpdatingTimestamp() *time.Time
	IsDeleting() bool
}

// ObjectList indicates a generic type of object list
type ObjectList interface {
	GetKind() string
	HasNamespace() bool
	AppendRaw([]byte) error
}

// Storage is the interface that is used for interacting with database.
type Storage interface {
	Close() error
	Create(obj Object) error
	Get(cv Object) error
	Watch(kind string) (Watcher, error)
	List(cv ObjectList, ns ...string) error
	Update(obj Object) error
	Delete(obj Object) error
}

// DefaultWatchChanSize indicates the default channel size of watcher output
const DefaultWatchChanSize = 100

// WatchEventType indicates the type of watching event
type WatchEventType byte

func (of WatchEventType) String() string {
	if v, ok := WatchEventTypeName[byte(of)]; ok {
		return v
	}
	return "<invalid>"
}

// WatchEventTypeName is used by stringer of WatchEventType
var WatchEventTypeName = map[byte]string{
	0x01: "CREATE",
	0x02: "UPDATE",
	0x04: "DELETE",
	0x08: "ERROR",
}

const (
	// CREATE indicates a creation watching event
	CREATE WatchEventType = 0x01 << iota
	// UPDATE indicates a updating watching event
	UPDATE
	// DELETE indicates a deletion watching event
	DELETE
	// ERROR indicates an error watching event
	ERROR
)

// Watcher is a generic watcher of Storage
type Watcher interface {
	Close() error
	Output() <-chan WatchEvent
}

// WatchEvent indicates a watching event that is sent by Storage.
type WatchEvent struct {
	Type  WatchEventType
	Kind  string
	Key   string
	Value []byte
}

// Unmarshal attempts to parse the value of event into given Object, and returns any encountered error.
func (in *WatchEvent) Unmarshal(cv Object) error {
	return json.Unmarshal(in.Value, cv)
}
