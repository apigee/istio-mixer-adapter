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

	"istio.io/istio/mixer/pkg/adapter/test"
)

func TestBucket(t *testing.T) {

	env := test.NewEnv(t)
	m := NewManager(url.URL{}, env)
	m.now = func() time.Time { return time.Unix(1521221450, 0) }

	priorRequests := []*request{
		{
			Weight: 1,
		},
		{
			Weight: 3,
		},
	}

	priorResult := &Result{
		Used: 2,
	}

	b := &bucket{
		org:         "org",
		env:         "env",
		id:          "id",
		requests:    priorRequests,
		result:      priorResult,
		created:     m.now(),
		lock:        sync.RWMutex{},
		now:         m.now,
		deleteAfter: defaultDeleteAfter,
	}

	req := request{
		Allow:  3,
		Weight: 2,
	}

	res := b.apply(m, &req)

	if res.Used != 8 {
		t.Errorf("Used got: %d, want: %d", res.Used, 8)
	}
	if res.Allowed != 3 {
		t.Errorf("Allowed got: %d, want: %d", res.Allowed, 3)
	}
	if res.Exceeded != 5 {
		t.Errorf("Exceeded got: %d, want: %d", res.Allowed, 5)
	}
	if b.checked != m.now() {
		t.Errorf("checked got: %v, want: %v", b.checked, m.now())
	}
}
