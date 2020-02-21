// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package analytics

import (
	"strings"
	"testing"
	"time"
)

func TestValidationFailure(t *testing.T) {
	ts := int64(1521221450) // This timestamp is roughly 11:30 MST on Mar. 16, 2018.
	for _, test := range []struct {
		desc      string
		record    Record
		wantError string
	}{
		{"good record", Record{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: ts * 1000,
			ClientReceivedEndTimestamp:   ts * 1000,
			GatewayFlowID:                "x",
		}, ""},
		{"missing org", Record{
			Environment:                  "test",
			ClientReceivedStartTimestamp: ts * 1000,
			ClientReceivedEndTimestamp:   ts * 1000,
			GatewayFlowID:                "x",
		}, "missing Organization"},
		{"missing env", Record{
			Organization:                 "hi",
			ClientReceivedStartTimestamp: ts * 1000,
			ClientReceivedEndTimestamp:   ts * 1000,
			GatewayFlowID:                "x",
		}, "missing Environment"},
		{"missing start timestamp", Record{
			Organization:               "hi",
			Environment:                "test",
			ClientReceivedEndTimestamp: ts * 1000,
			GatewayFlowID:              "x",
		}, "missing ClientReceivedStartTimestamp"},
		{"missing end timestamp", Record{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: ts * 1000,
			GatewayFlowID:                "x",
		}, "missing ClientReceivedEndTimestamp"},
		{"end < start", Record{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: ts * 1000,
			ClientReceivedEndTimestamp:   ts*1000 - 1,
			GatewayFlowID:                "x",
		}, "ClientReceivedStartTimestamp > ClientReceivedEndTimestamp"},
		{"in the future", Record{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: (ts + 1) * 1000,
			ClientReceivedEndTimestamp:   (ts + 1) * 1000,
			GatewayFlowID:                "x",
		}, "in the future"},
		{"too old", Record{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: (ts - 91*24*3600) * 1000,
			ClientReceivedEndTimestamp:   (ts - 91*24*3600) * 1000,
			GatewayFlowID:                "x",
		}, "more than 90 days old"},
		{"missing GatewayFlowID", Record{
			Organization:                 "hi",
			Environment:                  "test",
			ClientReceivedStartTimestamp: ts * 1000,
			ClientReceivedEndTimestamp:   ts * 1000,
		}, "missing GatewayFlowID"},
	} {
		t.Log(test.desc)

		gotErr := test.record.validate(time.Unix(ts, 0))
		if test.wantError == "" {
			if gotErr != nil {
				t.Errorf("got error %s, want none", gotErr)
			}
			continue
		}
		if gotErr == nil {
			t.Errorf("got nil error, want one containing %s", test.wantError)
			continue
		}

		if !strings.Contains(gotErr.Error(), test.wantError) {
			t.Errorf("error %s should contain '%s'", gotErr, test.wantError)
		}
	}
}
