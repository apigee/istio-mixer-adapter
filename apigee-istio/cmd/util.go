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

package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseString(s string) (interface{}, error)  { return s, nil }
func parseInt64(s string) (interface{}, error)   { return strconv.ParseInt(s, 10, 64) }
func parseFloat64(s string) (interface{}, error) { return strconv.ParseFloat(s, 64) }
func parseBool(s string) (interface{}, error)    { return strconv.ParseBool(s) }

func parseTime(s string) (interface{}, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func parseDuration(s string) (interface{}, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func parseBytes(s string) (interface{}, error) {
	var bytes []uint8
	for _, seg := range strings.Split(s, ":") {
		b, err := strconv.ParseUint(seg, 16, 8)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, uint8(b))
	}
	return bytes, nil
}

func parseStringMap(s string) (interface{}, error) {
	m := make(map[string]string)
	for _, pair := range strings.Split(s, ";") {
		colon := strings.Index(pair, ":")
		if colon < 0 {
			return nil, fmt.Errorf("%s is not a valid key/value pair in the form key:value", pair)
		}

		k := pair[0:colon]
		v := pair[colon+1:]
		m[k] = v
	}

	return m, nil
}

func parseAny(s string) (interface{}, error) {
	// auto-sense the type of attributes based on being able to parse the value
	if val, err := parseInt64(s); err == nil {
		return val, nil
	} else if val, err := parseFloat64(s); err == nil {
		return val, nil
	} else if val, err := parseBool(s); err == nil {
		return val, nil
	} else if val, err := parseTime(s); err == nil {
		return val, nil
	} else if val, err := parseDuration(s); err == nil {
		return val, nil
	} else if val, err := parseBytes(s); err == nil {
		return val, nil
	} else if val, err := parseStringMap(s); err == nil {
		return val, nil
	}
	return s, nil
}

type convertFn func(string) (interface{}, error)
