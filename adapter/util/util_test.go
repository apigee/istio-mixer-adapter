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

package util

import (
	"strings"
	"testing"

	"istio.io/istio/mixer/template/authorization"
)

func TestSprintfRedacted(t *testing.T) {

	superman := "Clark Kent"
	batman := "Bruce Wayne"
	ironman := "Tony Stark"

	inst := &authorization.Instance{
		Subject: &authorization.Subject{
			Properties: map[string]interface{}{
				"superman": superman,
				"ironman":  ironman,
				"batman":   batman,
			},
		},
	}

	redacts := []interface{}{superman, batman}
	result := SprintfRedacts(redacts, "%#v", *inst.Subject)

	if strings.Contains(result, superman) {
		t.Errorf("should not contain %s, got: %s", superman, result)
	}
	if strings.Contains(result, batman) {
		t.Errorf("should not contain %s, got: %s", batman, result)
	}
	if !strings.Contains(result, ironman) {
		t.Errorf("should contain %s, got: %s", ironman, result)
	}
}

func TestTruncate(t *testing.T) {
	for _, ea := range []struct {
		in   string
		end  int
		want string
	}{
		{"hello world", 5, "hello..."},
		{"hello", 5, "hello"},
		{"he", 5, "he"},
	} {
		t.Logf("in: '%s' end: %d", ea.in, ea.end)
		got := Truncate(ea.in, 5)
		if got != ea.want {
			t.Errorf("want: '%s', got: '%s'", ea.want, got)
		}
	}
}
