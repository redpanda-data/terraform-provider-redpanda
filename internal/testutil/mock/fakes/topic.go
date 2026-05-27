// Copyright 2026 Redpanda Data, Inc.
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package fakes

import (
	"context"
	"strings"
	"sync"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type topicRecord struct {
	name              string
	partitionCount    int32
	replicationFactor int32
	configs           map[string]string
}

// TopicFake is a stateful in-memory implementation of the TopicService RPCs
// the provider uses (Create/List/Delete/GetConfigurations/SetConfigurations/
// SetPartitions). The remaining 4 RPCs inherit Unimplemented.
type TopicFake struct {
	dataplanev1grpc.UnimplementedTopicServiceServer

	mu             sync.Mutex
	store          map[string]*topicRecord
	serverInjected map[string]string
}

// NewTopicFake returns an empty TopicFake.
func NewTopicFake() *TopicFake {
	return &TopicFake{
		store:          map[string]*topicRecord{},
		serverInjected: map[string]string{},
	}
}

// SetServerInjectedConfig registers a config entry the fake will append to
// every GetTopicConfigurations response (with DYNAMIC source) unless the
// caller has set the same key via SetTopicConfigurations. Mirrors the
// post-v26.1.1 broker behavior of injecting `redpanda.storage.mode = "unset"`
// on every topic — used to exercise the redpanda.* strip branch of
// mergeWithPlannedConfig.
func (f *TopicFake) SetServerInjectedConfig(key, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.serverInjected[key] = value
}

// CreateTopic stores a new topic. AlreadyExists if the name is taken.
func (f *TopicFake) CreateTopic(_ context.Context, req *dataplanev1.CreateTopicRequest) (*dataplanev1.CreateTopicResponse, error) {
	t := req.GetTopic()
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.store[t.GetName()]; ok {
		return nil, status.Errorf(codes.AlreadyExists, "TOPIC_ALREADY_EXISTS: %s", t.GetName())
	}
	pc := t.GetPartitionCount()
	if !t.HasPartitionCount() {
		pc = 3
	}
	rf := t.GetReplicationFactor()
	if !t.HasReplicationFactor() {
		rf = 1
	}
	rec := &topicRecord{
		name:              t.GetName(),
		partitionCount:    pc,
		replicationFactor: rf,
		configs:           map[string]string{},
	}
	for _, c := range t.GetConfigs() {
		rec.configs[c.GetName()] = c.GetValue()
	}
	f.store[rec.name] = rec
	return &dataplanev1.CreateTopicResponse{
		TopicName:         rec.name,
		PartitionCount:    rec.partitionCount,
		ReplicationFactor: rec.replicationFactor,
	}, nil
}

// ListTopics returns topics matching the NameContains substring filter.
// Empty filter matches all topics.
func (f *TopicFake) ListTopics(_ context.Context, req *dataplanev1.ListTopicsRequest) (*dataplanev1.ListTopicsResponse, error) {
	nameContains := req.GetFilter().GetNameContains()
	f.mu.Lock()
	defer f.mu.Unlock()
	var topics []*dataplanev1.ListTopicsResponse_Topic
	for _, rec := range f.store {
		if nameContains != "" && !strings.Contains(rec.name, nameContains) {
			continue
		}
		topics = append(topics, &dataplanev1.ListTopicsResponse_Topic{
			Name:              rec.name,
			PartitionCount:    rec.partitionCount,
			ReplicationFactor: rec.replicationFactor,
		})
	}
	return &dataplanev1.ListTopicsResponse{Topics: topics}, nil
}

// DeleteTopic removes the topic; NotFound if absent.
func (f *TopicFake) DeleteTopic(_ context.Context, req *dataplanev1.DeleteTopicRequest) (*dataplanev1.DeleteTopicResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.store[req.GetTopicName()]; !ok {
		return nil, status.Errorf(codes.NotFound, "TOPIC_DOES_NOT_EXIST: %s", req.GetTopicName())
	}
	delete(f.store, req.GetTopicName())
	return &dataplanev1.DeleteTopicResponse{}, nil
}

// GetTopicConfigurations returns user-managed configs as dynamic-source
// entries, plus any keys registered via SetServerInjectedConfig that the
// user hasn't set themselves.
func (f *TopicFake) GetTopicConfigurations(_ context.Context, req *dataplanev1.GetTopicConfigurationsRequest) (*dataplanev1.GetTopicConfigurationsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.store[req.GetTopicName()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "TOPIC_DOES_NOT_EXIST: %s", req.GetTopicName())
	}
	cfgs := topicConfigsToProto(rec)
	for k, v := range f.serverInjected {
		if _, set := rec.configs[k]; set {
			continue
		}
		val := v
		cfgs = append(cfgs, &dataplanev1.Topic_Configuration{
			Name:   k,
			Value:  &val,
			Source: dataplanev1.ConfigSource_CONFIG_SOURCE_DYNAMIC_TOPIC_CONFIG,
		})
	}
	return &dataplanev1.GetTopicConfigurationsResponse{Configurations: cfgs}, nil
}

// SetTopicConfigurations replaces the topic's config map.
func (f *TopicFake) SetTopicConfigurations(_ context.Context, req *dataplanev1.SetTopicConfigurationsRequest) (*dataplanev1.SetTopicConfigurationsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.store[req.GetTopicName()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "TOPIC_DOES_NOT_EXIST: %s", req.GetTopicName())
	}
	rec.configs = map[string]string{}
	for _, c := range req.GetConfigurations() {
		rec.configs[c.GetName()] = c.GetValue()
	}
	return &dataplanev1.SetTopicConfigurationsResponse{Configurations: topicConfigsToProto(rec)}, nil
}

// SetTopicPartitions updates the partition count.
func (f *TopicFake) SetTopicPartitions(_ context.Context, req *dataplanev1.SetTopicPartitionsRequest) (*dataplanev1.SetTopicPartitionsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.store[req.GetTopicName()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "TOPIC_DOES_NOT_EXIST: %s", req.GetTopicName())
	}
	rec.partitionCount = req.GetPartitionCount()
	return &dataplanev1.SetTopicPartitionsResponse{}, nil
}

func topicConfigsToProto(rec *topicRecord) []*dataplanev1.Topic_Configuration {
	cfgs := make([]*dataplanev1.Topic_Configuration, 0, len(rec.configs))
	for k, v := range rec.configs {
		val := v
		cfgs = append(cfgs, &dataplanev1.Topic_Configuration{
			Name:   k,
			Value:  &val,
			Source: dataplanev1.ConfigSource_CONFIG_SOURCE_DYNAMIC_TOPIC_CONFIG,
		})
	}
	return cfgs
}
