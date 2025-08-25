package service

import "errors"

var (
	// ErrEmptyInput is returned when input is empty
	ErrEmptyInput = errors.New("input cannot be empty")
	
	// ErrInvalidInput is returned when input is invalid
	ErrInvalidInput = errors.New("invalid input provided")
	
	// ErrProcessingFailed is returned when processing fails
	ErrProcessingFailed = errors.New("processing failed")
	
	// ErrNotFound is returned when a resource is not found
	ErrNotFound = errors.New("resource not found")
)
