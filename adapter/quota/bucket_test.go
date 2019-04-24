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
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestBucket(t *testing.T) {
	now := func() time.Time { return time.Unix(1521221450, 0) }
	m := &Manager{now: now}

	cases := map[string]struct {
		priorRequest *Request
		priorResult  *Result
		request      *Request
		want         *Result
	}{
		"First request": {
			&Request{
				Allow: 3,
			},
			nil,
			&Request{
				Allow:  3,
				Weight: 2,
			},
			&Result{
				Allowed:    3,
				Used:       2,
				Exceeded:   0,
				ExpiryTime: now().Unix(),
				Timestamp:  now().Unix(),
			},
		},
		"Valid request": {
			&Request{
				Allow:  4,
				Weight: 1,
			},
			&Result{Used: 2},
			&Request{
				Allow:  4,
				Weight: 1,
			},
			&Result{
				Allowed:    4,
				Used:       4,
				Exceeded:   0,
				ExpiryTime: now().Unix(),
				Timestamp:  now().Unix(),
			},
		},
		"Newly exceeded": {
			&Request{
				Allow:  7,
				Weight: 3,
			},
			&Result{Used: 3},
			&Request{
				Allow:  7,
				Weight: 2,
			},
			&Result{
				Allowed:    7,
				Used:       7,
				Exceeded:   1,
				ExpiryTime: now().Unix(),
				Timestamp:  now().Unix(),
			},
		},
		"Previously exceeded": {
			&Request{
				Allow: 3,
			},
			&Result{
				Used:     3,
				Exceeded: 1,
			},
			&Request{
				Allow:  3,
				Weight: 1,
			},
			&Result{
				Allowed:    3,
				Used:       3,
				Exceeded:   2,
				ExpiryTime: now().Unix(),
				Timestamp:  now().Unix(),
			},
		},
	}

	for id, c := range cases {
		t.Logf("** Executing test case '%s' **", id)

		b := &bucket{
			manager:     m,
			request:     c.priorRequest,
			result:      c.priorResult,
			created:     now(),
			lock:        sync.RWMutex{},
			deleteAfter: defaultDeleteAfter,
		}

		res, err := b.apply(c.request)
		if err != nil {
			t.Errorf("should not get error: %v", err)
		}

		if !reflect.DeepEqual(res, c.want) {
			t.Errorf("got: %#v, want: %#v", res, c.want)
		}
	}
}

func TestNeedToDelete(t *testing.T) {
	now := func() time.Time { return time.Unix(1521221450, 0) }
	m := &Manager{now: now}

	cases := map[string]struct {
		request *Request
		checked time.Time
		want    bool
	}{
		"empty": {
			request: &Request{},
			want:    true,
		},
		"recently checked": {
			request: &Request{},
			checked: now(),
			want:    false,
		},
		"not recently checked": {
			request: &Request{},
			checked: now().Add(-time.Hour),
			want:    true,
		},
		"has pending requests": {
			request: &Request{Weight: 1},
			checked: now().Add(-time.Hour),
			want:    false,
		},
	}

	for id, c := range cases {
		t.Logf("** Executing test case '%s' **", id)
		b := bucket{
			manager:     m,
			deleteAfter: time.Minute,
			request:     c.request,
			checked:     c.checked,
		}
		if c.want != b.needToDelete() {
			t.Errorf("want: %v got: %v", c.want, b.needToDelete())
		}
	}
}

func TestNeedToSync(t *testing.T) {
	now := func() time.Time { return time.Unix(1521221450, 0) }
	m := &Manager{now: now}

	cases := map[string]struct {
		request *Request
		synced  time.Time
		want    bool
	}{
		"empty": {
			request: &Request{},
			want:    true,
		},
		"recently synced": {
			request: &Request{},
			synced:  now(),
			want:    false,
		},
		"not recently synced": {
			request: &Request{},
			synced:  now().Add(-time.Hour),
			want:    true,
		},
		"has pending requests": {
			request: &Request{Weight: 1},
			synced:  now(),
			want:    true,
		},
	}

	for id, c := range cases {
		t.Logf("** Executing test case '%s' **", id)
		b := bucket{
			manager:      m,
			refreshAfter: time.Minute,
			request:      c.request,
			synced:       c.synced,
		}
		if c.want != b.needToSync() {
			t.Errorf("want: %v got: %v", c.want, b.needToDelete())
		}
	}
}
