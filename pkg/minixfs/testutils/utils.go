package testutils

import (
	"fmt"
	"runtime"
	"testing"
)

func ErrorLevel(test *testing.T, level int, str string, args ...interface{}) {
	_, file, line, _ := runtime.Caller(level)
	info := fmt.Sprintf("[%s:%d] ", file, line)
	test.Errorf(info + str, args...)
}

func ErrorHere(test *testing.T, str string, args ...interface{}) {
	ErrorLevel(test, 2, str, args...)
}

func FatalHere(test *testing.T, str string, args ...interface{}) {
	_, file, line, _ := runtime.Caller(1)
	info := fmt.Sprintf("[%s:%d] ", file, line)
	test.Fatalf(info + str, args...)
}
