package app

import (
	"fmt"
)

func validateKey(keyBuf []byte, key string, max int64) (bool, string) {
	var msg string
	if key == "" {
		msg = "empty key"
		return false, msg
	}

	keyBuf = append(keyBuf, key...)
	if int64(len(keyBuf)) < max {
		return true, ""
	}

	msg = fmt.Sprintf(
		"entry key size: %d is bigger than max key size in bytes:%d",
		len(keyBuf), max)

	return false, msg
}

func validateValue(value []byte, max int64) (bool, string) {
	var msg string
	valueLen := len(value)
	if valueLen == 0 {
		msg := "value is empty"
		return false, msg
	}

	if int64(valueLen) < max {
		return true, ""
	}

	msg = fmt.Sprintf(
		"entry value size: %d is bigger than max value size in bytes:%d",
		valueLen, max)

	return false, msg
}
