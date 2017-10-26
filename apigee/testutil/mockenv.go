package testutil

import (
	"istio.io/mixer/pkg/adapter"
	"fmt"
)

func MakeMockEnv() mockEnv {
	return mockEnv{
		mockLog{},
	}
}

type mockLog struct {
}
func (l mockLog) VerbosityLevel(level adapter.VerbosityLevel) bool {
	return true
}
func (l mockLog) Infof(format string, args ...interface{}) {
	fmt.Printf(format, args)
}
func (l mockLog) Warningf(format string, args ...interface{}) {
	fmt.Printf(format, args)
}
func (l mockLog) Errorf(format string, args ...interface{}) error {
	fmt.Printf(format, args)
	return nil
}

type mockEnv struct {
	logger adapter.Logger
}
func (e mockEnv) Logger() adapter.Logger {
	return e.logger
}
func (e mockEnv) ScheduleWork(fn adapter.WorkFunc) {
	panic("not implemented")
}
func (e mockEnv) ScheduleDaemon(fn adapter.DaemonFunc) {
	panic("not implemented")
}
