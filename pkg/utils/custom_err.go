package utils

import "errors"

var (
	ErrTagNotFound            = errors.New("tag not found")
	ErrInvalidPage            = errors.New("invalid page parameter")
	ErrInvalidPageSize        = errors.New("invalid page size parameter")
	ErrDatabaseError          = errors.New("database error")
	ErrPOINotFound            = errors.New("poi not found")
	ErrUnexpectedBehaviorOfAI = errors.New("unexpected error from AI service")
	ErrInvalidInput           = errors.New("invalid input")
	ErrPoorQualityInput       = errors.New("input quality is too low please consider improving it so we can help you better")
)
