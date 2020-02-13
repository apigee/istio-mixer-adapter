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
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

// SprintfRedacts truncates secret strings to len(5)
func SprintfRedacts(redacts []interface{}, format string, a ...interface{}) string {
	s := fmt.Sprintf(format, a...)
	for _, r := range redacts {
		if r, ok := r.(string); ok {
			truncated := Truncate(r, 5)
			s = strings.Replace(s, r, truncated, -1)
		}
	}
	return s
}

// Truncate truncates secret strings to arbitrary length and adds "..." as indication
func Truncate(in string, end int) string {
	out := in
	if len(out) > end {
		out = fmt.Sprintf("%s...", out[0:end])
	}
	return out
}

// ReadPropertiesFile Parses a simple properties file (xx=xx format)
func ReadPropertiesFile(fileName string) (map[string]string, error) {
	config := map[string]string{}
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		text := scan.Text()
		if eq := strings.Index(text, "="); eq >= 0 {
			if key := strings.TrimSpace(text[:eq]); len(key) > 0 {
				value := ""
				if len(text) > eq {
					value = strings.TrimSpace(text[eq+1:])
				}
				config[key] = value
			}
		}
	}

	if err := scan.Err(); err != nil {
		return nil, err
	}

	return config, nil
}

// FreePort returns a free port number
func FreePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}

	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return 0, err
	}
	return port, nil
}
