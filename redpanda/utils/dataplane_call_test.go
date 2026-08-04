// Copyright 2025 Redpanda Data, Inc.
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

package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestDataplaneCall(t *testing.T) {
	SetTestModeWaits()

	bareUnknown := grpcstatus.Error(codes.Unknown, "")
	alreadyExists := grpcstatus.Error(codes.AlreadyExists, "already exists")
	invalid := grpcstatus.Error(codes.InvalidArgument, "bad name")

	for _, tt := range []struct {
		name string
		// errs is returned in order, one per attempt; nil means success.
		errs []error
		// probeFrom is the attempt from which the probe starts finding the
		// resource; 0 disables the probe entirely.
		probeFrom int
		wantCalls int
		wantValue string
		wantErr   bool
	}{
		{
			name:      "success on the first attempt",
			errs:      []error{nil},
			wantCalls: 1,
			wantValue: "created",
		},
		{
			name:      "non-transient fails without retrying",
			errs:      []error{invalid},
			wantCalls: 1,
			wantErr:   true,
		},
		{
			// The warm-up window: bare UNKNOWN says nothing about the request.
			name:      "transient retries until it succeeds",
			errs:      []error{bareUnknown, bareUnknown, nil},
			wantCalls: 3,
			wantValue: "created",
		},
		{
			// The hazard that let one stack silently take over another's secret.
			name:      "AlreadyExists on the first attempt fails",
			errs:      []error{alreadyExists},
			probeFrom: 1,
			wantCalls: 1,
			wantErr:   true,
		},
		{
			// Here the earlier attempt genuinely landed, so adopting is right.
			name:      "AlreadyExists after a retry adopts via the probe",
			errs:      []error{bareUnknown, alreadyExists},
			probeFrom: 2,
			wantCalls: 2,
			wantValue: "probed",
		},
		{
			name:      "AlreadyExists after a retry fails when the probe finds nothing",
			errs:      []error{bareUnknown, alreadyExists},
			probeFrom: 0,
			wantCalls: 2,
			wantErr:   true,
		},
		{
			// A transient failure whose call landed anyway.
			name:      "transient adopts via the probe rather than retrying forever",
			errs:      []error{bareUnknown, bareUnknown},
			probeFrom: 2,
			wantCalls: 2,
			wantValue: "probed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			do := func(context.Context) (string, error) {
				calls++
				err := tt.errs[min(calls, len(tt.errs))-1]
				if err != nil {
					return "", err
				}
				return "created", nil
			}

			var opts []CallOption[string]
			if tt.probeFrom > 0 {
				opts = append(opts, WithProbe(func(context.Context) (string, bool) {
					if calls >= tt.probeFrom {
						return "probed", true
					}
					return "", false
				}))
			}

			got, err := DataplaneCall(context.Background(), do, opts...)

			assert.Equal(t, tt.wantCalls, calls, "attempts")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantValue, got)
		})
	}
}

// TestDataplaneCallProbeNeverRunsOnFirstAttempt pins the rule the AlreadyExists
// and transient branches both depend on: nothing the call did could have created
// anything before its first attempt, so a probe hit then belongs to someone else.
func TestDataplaneCallProbeNeverRunsOnFirstAttempt(t *testing.T) {
	SetTestModeWaits()

	probed := 0
	_, err := DataplaneCall(context.Background(),
		func(context.Context) (string, error) {
			return "", grpcstatus.Error(codes.AlreadyExists, "already exists")
		},
		WithProbe(func(context.Context) (string, bool) {
			probed++
			return "adopted", true
		}),
	)

	require.Error(t, err, "a first-attempt AlreadyExists must not be adopted")
	assert.Zero(t, probed, "the probe must not run on the first attempt")
}

func TestDataplaneCallOptions(t *testing.T) {
	SetTestModeWaits()

	t.Run("a custom classifier decides what is transient", func(t *testing.T) {
		calls := 0
		_, err := DataplaneCall(context.Background(),
			func(context.Context) (string, error) {
				calls++
				if calls < 2 {
					// Not transient under the default gRPC classifier.
					return "", errors.New("registry not ready")
				}
				return "ok", nil
			},
			WithClassifier[string](func(err error) bool {
				return err != nil && err.Error() == "registry not ready"
			}),
		)
		require.NoError(t, err)
		assert.Equal(t, 2, calls, "the custom classifier should have allowed a retry")
	})

	t.Run("DataplaneCallOnce does not retry", func(t *testing.T) {
		calls := 0
		_, err := DataplaneCallOnce(context.Background(), func(context.Context) (string, error) {
			calls++
			return "", grpcstatus.Error(codes.Unknown, "")
		})
		require.Error(t, err)
		assert.Equal(t, 1, calls, "the opt-out must issue exactly one call")
	})
}
