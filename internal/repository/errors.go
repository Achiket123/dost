package repository

import "errors"

var (
	// ErrDataNotFound is returned when data is not found
	ErrDataNotFound = errors.New("data not found")
	
	// ErrDataExists is returned when data already exists
	ErrDataExists = errors.New("data already exists")
	
	// ErrInvalidData is returned when data is invalid
	ErrInvalidData = errors.New("invalid data")
)
