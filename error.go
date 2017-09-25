package grpclb

import "errors"

var (
	// ErrNoServer when no server has found then return error.
	ErrNoServer = errors.New("server selector: There is no server")
	// ErrServerExisted the server has existed when add to the ServerSelector.
	ErrServerExisted = errors.New("server selector: Server has existed")
	// ErrServerNotExisted the server has not existed when delete from ServerSelector.
	ErrServerNotExisted = errors.New("server selector: Server has not existed")
	// ErrUnsupportOp operation must be in Add or Delete
	ErrUnsupportOp = errors.New("server selector: Unsupport operation, must be one of Add or Delete")
)
