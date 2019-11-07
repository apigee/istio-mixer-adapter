// Copyright 2019 Google LLC
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

package util_test

import (
	"sync"
	"testing"

	"github.com/apigee/istio-mixer-adapter/adapter/util"
	"istio.io/istio/mixer/pkg/adapter/test"
)

func TestReservoirStream(t *testing.T) {
	const limit = 50
	env := test.NewEnv(t)

	in, out, _ := util.NewReservoir(env, limit)

	lastVal := -1
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for v := range out {
			vi := v.(int)
			t.Logf("receive: %d", vi)
			if lastVal+1 != vi {
				t.Errorf("want %d, got %d", lastVal+1, vi)
			}
			lastVal = vi
		}
		wg.Done()
	}()

	for i := 0; i < 20; i++ {
		t.Logf("send: %d", i)
		in <- i
	}
	close(in)
	wg.Wait()

	if lastVal != 19 {
		t.Errorf("lastVal: %d, got %d", lastVal, 19)
	}
}

func TestReservoirLimit(t *testing.T) {
	const limit = 2
	env := test.NewEnv(t)

	in, out, overflow := util.NewReservoir(env, limit)

	for i := 0; i < 10; i++ {
		t.Logf("send: %d", i)
		in <- i
	}
	i := (<-out).(int)
	if i != 0 {
		t.Errorf("want %d, got %d", 0, i)
	}
	i = (<-overflow).(int)
	if i != 2 {
		t.Errorf("want %d, got %d", 2, i)
	}

	close(in)

	i = (<-out).(int)
	if i != 1 {
		t.Errorf("want %d, got %d", 1, i)
	}

	_, ok := <-out
	if ok {
		t.Errorf("channel should be closed")
	}

	i, ok = (<-overflow).(int)
	if ok {
		t.Errorf("channel should be closed")
	}
	if i != 0 {
		t.Errorf("want %d, got %d", 0, i)
	}
}
