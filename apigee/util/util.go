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
	"strings"
)

func SprintfRedacts(redacts []interface{}, format string, a ...interface{}) string {
	s := fmt.Sprintf(format, a)
	for _, r := range redacts {
		if r, ok := r.(string); ok {
			truncated := Truncate(r, 5)
			s = strings.Replace(s, r, truncated, -1)
		}
	}
	return s
}

func Truncate(in string, end int) string {
	out := in
	if len(out) > end {
		out = fmt.Sprintf("%s...", out[0:end])
	}
	return out
}
