package utils

import "errors"

var (
	ErrTagNotFound     = errors.New("tag not found")
	ErrInvalidPage     = errors.New("invalid page parameter")
	ErrInvalidPageSize = errors.New("invalid page size parameter")
	ErrDatabaseError   = errors.New("database error")
)
