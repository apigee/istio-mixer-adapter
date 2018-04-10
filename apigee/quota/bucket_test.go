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

package quota

import (
	"net/url"
	"sync"
	"testing"
	"time"

	"reflect"
)

func TestBucket(t *testing.T) {
	now := func() time.Time { return time.Unix(1521221450, 0) }

	cases := map[string]struct {
		priorRequests []*Request
		priorResult   *Result
		request       *Request
		want          Result
	}{
		"First request": {
			nil,
			nil,
			&Request{
				Allow:  3,
				Weight: 2,
			},
			Result{
				Allowed:    3,
				Used:       2,
				Exceeded:   0,
				ExpiryTime: now().Unix(),
				Timestamp:  now().Unix(),
			},
		},
		"Valid request": {
			[]*Request{
				{Weight: 1},
			},
			&Result{Used: 2},
			&Request{
				Allow:  4,
				Weight: 1,
			},
			Result{
				Allowed:    4,
				Used:       4,
				Exceeded:   0,
				ExpiryTime: now().Unix(),
				Timestamp:  now().Unix(),
			},
		},
		"Newly exceeded": {
			[]*Request{
				{Weight: 1},
				{Weight: 2},
			},
			&Result{Used: 3},
			&Request{
				Allow:  7,
				Weight: 2,
			},
			Result{
				Allowed:    7,
				Used:       7,
				Exceeded:   1,
				ExpiryTime: now().Unix(),
				Timestamp:  now().Unix(),
			},
		},
		"Previously exceeded": {
			[]*Request{},
			&Result{
				Used:     3,
				Exceeded: 1,
			},
			&Request{
				Allow:  3,
				Weight: 1,
			},
			Result{
				Allowed:    3,
				Used:       3,
				Exceeded:   2,
				ExpiryTime: now().Unix(),
				Timestamp:  now().Unix(),
			},
		},
	}

	m := newManager(url.URL{})

	for id, c := range cases {
		t.Logf("** Executing test case '%s' **", id)

		b := &bucket{
			quotaURL:    "",
			id:          "id",
			requests:    c.priorRequests,
			result:      c.priorResult,
			created:     now(),
			lock:        sync.RWMutex{},
			now:         now,
			deleteAfter: defaultDeleteAfter,
		}

		res := b.apply(m, c.request)

		if !reflect.DeepEqual(res, c.want) {
			t.Errorf("got: %#v, want: %#v", res, c.want)
		}
	}
}
