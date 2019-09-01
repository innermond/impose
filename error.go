package main

import (
	"bytes"
	"fmt"
)

// direct copy from https://middlemost.com/failure-is-your-domain/
// Err cannot coexist with Code or Message
type Error struct {
	Code    string
	Message string
	Op      string
	Err     error
}

// Error returns the string representation of the error message.
func (e *Error) Error() string {
	var buf bytes.Buffer

	// Print the current operation in our stack, if any.
	if e.Op != "" {
		fmt.Fprintf(&buf, "%s: ", e.Op)
	}

	// If wrapping an error, print its Error() message.
	// Otherwise print the error code & message.
	if e.Err != nil {
		buf.WriteString(e.Err.Error())
	} else {
		if e.Code != "" {
			fmt.Fprintf(&buf, "<%s> ", e.Code)
		}
		buf.WriteString(e.Message)
	}
	return buf.String()
}

// Application error codes
const (
	ECONFLICT = "conflict"
	EINTERNAL = "internal"
	EINVALID  = "invalid"
	ENOTFOUND = "not_found"
)

func ErrorCode(err error) string {
	if err == nil {
		return ""
	}

	if e, ok := err.(*Error); ok && e.Code != "" {
		return e.Code
	} else if ok && e.Err != nil {
		return ErrorCode(e.Err)
	}

	return EINTERNAL
}

func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	if e, ok := err.(*Error); ok && e.Message != "" {
		return e.Message
	} else if ok && e.Err != nil {
		return ErrorMessage(e.Err)
	}

	return "An internal error has occured"
}
