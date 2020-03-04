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
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"testing"
)

func TestSprintfRedacted(t *testing.T) {

	superman := "Clark Kent"
	batman := "Bruce Wayne"
	ironman := "Tony Stark"

	test := map[string]interface{}{
		"superman": superman,
		"ironman":  ironman,
		"batman":   batman,
	}

	redacts := []interface{}{superman, batman}
	result := SprintfRedacts(redacts, "%#v", test)

	if strings.Contains(result, superman) {
		t.Errorf("should not contain %s, got: %s", superman, result)
	}
	if !strings.Contains(result, "Clark...") {
		t.Errorf("should contain %s, got: %s", "Clark...", result)
	}
	if strings.Contains(result, batman) {
		t.Errorf("should not contain %s, got: %s", batman, result)
	}
	if !strings.Contains(result, "Bruce...") {
		t.Errorf("should contain %s, got: %s", "Bruce...", result)
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

func TestReadPropertiesFile(t *testing.T) {
	tf, err := ioutil.TempFile("", "properties")
	if err != nil {
		t.Fatalf("TempFile: %v", err)
	}
	defer os.Remove(tf.Name())

	sourceMap := map[string]string{
		"a.valid.port": "apigee-udca-theganyo-apigee-test.apigee.svc.cluster.local:20001",
		"a.valid.url":  "https://apigee-synchronizer-theganyo-apigee-test.apigee.svc.cluster.local:8843/v1/versions/active/zip",
	}
	for k, v := range sourceMap {
		line := fmt.Sprintf("%s=%s\n", k, v)
		if _, err := tf.WriteString(line); err != nil {
			log.Fatal(err)
		}
	}
	if err := tf.Close(); err != nil {
		log.Fatal(err)
	}
	props, err := ReadPropertiesFile(tf.Name())
	if err != nil {
		t.Fatalf("ReadPropertiesFile: %v", err)
	}

	for k, v := range sourceMap {
		if props[k] != v {
			t.Errorf("expected: %s at key: %s, got: %s", v, k, props[k])
		}
	}
}

func TestFreeport(t *testing.T) {
	p, err := FreePort()
	if err != nil {
		t.Errorf("shouldn't get error: %v", err)
	}

	addr := fmt.Sprintf("localhost:%d", p)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Errorf("shouldn't get error: %v", err)
	}

	l.Close()
}
