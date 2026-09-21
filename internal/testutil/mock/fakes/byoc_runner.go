// Copyright 2026 Redpanda Data, Inc.
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
	"errors"
	"sync"
	"time"
)

// AgentState is the cluster's agent phase observed by the byoc runner fake at
// the moment of a call.
type AgentState int

// Agent phases the cluster fake reports to the byoc runner fake.
const (
	AgentStateNone     AgentState = iota // cluster missing or not in an agent phase
	AgentStateCreating                   // STATE_CREATING_AGENT
	AgentStateReady                      // STATE_READY
	AgentStateDeleting                   // STATE_DELETING_AGENT
)

// ByocCall records one RunByoc invocation.
type ByocCall struct {
	ClusterID   string
	Verb        string
	StateAtCall AgentState
	// SinkLines is how many of Lines the caller's sink received; zero when
	// the caller passed no sink.
	SinkLines int
}

// ByocRunnerFake stands in for utils.ByocClient. Each run is recorded and
// handed to advance, which the mock server wires to ClusterFake.AgentRun so
// the cluster moves through its agent phase the way the real plugin's apply
// and destroy make the control plane move it.
type ByocRunnerFake struct {
	mu      sync.Mutex
	calls   []ByocCall
	advance func(clusterID, verb string) (AgentState, error)

	// Lines are emitted to the sink on every run, in order, before advance.
	Lines []string
	// Err, when set, is returned from every run after recording it and
	// without advancing the cluster.
	Err error
	// nextErrs are returned one per run, in order, before Err is consulted.
	nextErrs []error
	// blockNext, when non-zero, makes only the next run wait until the
	// context ends or blockNext elapses, whichever is first.
	blockNext time.Duration
}

// BlockNextFor makes the next run behave like a hung plugin: it waits until
// the context ends, returning ctx.Err(), or until d elapses, returning
// ErrBlockedRunGaveUp because the caller never bounded the run. Later runs
// proceed normally, so the end-of-case destroy is unaffected.
func (f *ByocRunnerFake) BlockNextFor(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blockNext = d
}

// ErrBlockedRunGaveUp is returned by a BlockNextFor run whose context outlived
// the block duration: the caller never bounded the run.
var ErrBlockedRunGaveUp = errors.New("byoc runner fake: blocked run gave up waiting for the context to end")

// FailNextWith queues err for the next run only; later runs proceed
// normally. Models a transient plugin failure such as "please retry later".
func (f *ByocRunnerFake) FailNextWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextErrs = append(f.nextErrs, err)
}

// NewByocRunnerFake returns a fake whose runs advance the cluster through
// advance.
func NewByocRunnerFake(advance func(clusterID, verb string) (AgentState, error)) *ByocRunnerFake {
	return &ByocRunnerFake{advance: advance}
}

// RunByoc implements utils.ByocRunner.
func (f *ByocRunnerFake) RunByoc(ctx context.Context, clusterID, verb string, sink func(string)) error {
	f.mu.Lock()
	lines := append([]string(nil), f.Lines...)
	err := f.Err
	block := f.blockNext
	f.blockNext = 0
	if len(f.nextErrs) > 0 {
		err, f.nextErrs = f.nextErrs[0], f.nextErrs[1:]
	}
	f.mu.Unlock()
	delivered := 0
	if sink != nil {
		for _, l := range lines {
			sink(l)
			delivered++
		}
	}
	if err == nil && block > 0 {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-time.After(block):
			err = ErrBlockedRunGaveUp
		}
	}
	if err != nil {
		f.record(ByocCall{ClusterID: clusterID, Verb: verb, StateAtCall: AgentStateNone, SinkLines: delivered})
		return err
	}
	state, advErr := f.advance(clusterID, verb)
	f.record(ByocCall{ClusterID: clusterID, Verb: verb, StateAtCall: state, SinkLines: delivered})
	return advErr
}

func (f *ByocRunnerFake) record(c ByocCall) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c)
}

// Calls returns every recorded run in order.
func (f *ByocRunnerFake) Calls() []ByocCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ByocCall(nil), f.calls...)
}
