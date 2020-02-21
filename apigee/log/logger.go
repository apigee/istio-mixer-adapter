// Copyright 2020 Google LLC
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

package log

import (
	"fmt"
	l "log"
)

// Log is the global logger
var Log Logger = &defaultLogger{}

// init() {
// 	Logger = defaultLogger{}
// }

// Infof logs at the info level
func Infof(format string, args ...interface{}) {
	Log.Infof(format, args)
}

// Warningf logs suspect situations and recoverable errors
func Warningf(format string, args ...interface{}) {
	Log.Warningf(format, args)
}

// Errorf logs error conditions.
// In addition to generating a log record for the error, this also returns
// an error instance for convenience.
func Errorf(format string, args ...interface{}) error {
	return Log.Errorf(format, args)
}

// Debugf logs potentially verbose debug-time data
func Debugf(format string, args ...interface{}) {
	Log.Warningf(format, args)
}

// InfoEnabled returns whether output of messages at the info level is currently enabled.
func InfoEnabled() bool {
	return Log.InfoEnabled()
}

// WarnEnabled returns whether output of messages at the warn level is currently enabled.
func WarnEnabled() bool {
	return Log.WarnEnabled()
}

// ErrorEnabled returns whether output of messages at the wanr level is currently enabled.
func ErrorEnabled() bool {
	return Log.ErrorEnabled()
}

// DebugEnabled returns whether output of messages at the debug level is currently enabled.
func DebugEnabled() bool {
	return Log.DebugEnabled()
}

// Logger is a logging interface
type Logger interface {
	// Debugf logs potentially verbose debug-time data
	Debugf(format string, args ...interface{})
	// Infof logs informational data
	Infof(format string, args ...interface{})
	// Warningf logs suspect situations and recoverable errors
	Warningf(format string, args ...interface{})
	// Errorf logs error conditions.
	// In addition to generating a log record for the error, this also returns
	// an error instance for convenience.
	Errorf(format string, args ...interface{}) error

	// DebugEnabled returns whether output of messages at the debug level is currently enabled.
	DebugEnabled() bool
	// InfoEnabled returns whether output of messages at the info level is currently enabled.
	InfoEnabled() bool
	// WarnEnabled returns whether output of messages at the warn level is currently enabled.
	WarnEnabled() bool
	// ErrorEnabled returns whether output of messages at the error level is currently enabled.
	ErrorEnabled() bool
}

type defaultLogger struct {
}

func (d *defaultLogger) Infof(format string, args ...interface{}) {
	l.Printf(format, args...)
}

// Warningf logs suspect situations and recoverable errors
func (d *defaultLogger) Warningf(format string, args ...interface{}) {
	l.Printf(format, args...)
}

// Errorf logs error conditions.
// In addition to generating a log record for the error, this also returns
// an error instance for convenience.
func (d *defaultLogger) Errorf(format string, args ...interface{}) error {
	e := fmt.Errorf(format, args...)
	l.Printf(e.Error())
	return e
}

// Debugf logs potentially verbose debug-time data
func (d *defaultLogger) Debugf(format string, args ...interface{}) {
	l.Printf(format, args...)
}

// InfoEnabled returns whether output of messages at the info level is currently enabled.
func (d *defaultLogger) InfoEnabled() bool {
	return true
}

// InfoEnabled returns whether output of messages at the warn level is currently enabled.
func (d *defaultLogger) WarnEnabled() bool {
	return true
}

// ErrorEnabled returns whether output of messages at the wanr level is currently enabled.
func (d *defaultLogger) ErrorEnabled() bool {
	return true
}

// DebugEnabled returns whether output of messages at the debug level is currently enabled.
func (d *defaultLogger) DebugEnabled() bool {
	return true
}
