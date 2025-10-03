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
	ErrUnauthorized           = errors.New("unauthorized")
	ErrUnauthenticated        = errors.New("unauthenticated")
	ErrAccountNotFound        = errors.New("account not found")
	ErrInvalidCredentials     = errors.New("user or password is incorrect")
	ErrEmailAlreadyExists     = errors.New("email already exists")
	ErrJourneyNotFound        = errors.New("journey not found")
	RecordNotFound            = errors.New("record not found")
	ErrThirdService           = errors.New("third service error")
	ErrInvalidToken           = errors.New("invalid token")
)
