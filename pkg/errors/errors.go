package errors

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrDocumentNotFound    = errors.New("document not found")
	ErrDocumentExists      = errors.New("document already exists")
	ErrShardUnavailable    = errors.New("shard unavailable")
	ErrInvalidInput        = errors.New("invalid input")
	ErrIdempotencyConflict = errors.New("idempotency key already used")
	ErrRateLimited         = errors.New("rate limit exceeded")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInternal            = errors.New("internal error")
	ErrTimeout             = errors.New("operation timed out")
)

type AppError struct {
	Err        error
	Message    string
	StatusCode int
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Err.Error(), e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(sentinel error, statusCode int, message string) *AppError {
	return &AppError{
		Err:        sentinel,
		Message:    message,
		StatusCode: statusCode,
	}
}

func Newf(sentinel error, statusCode int, format string, args ...any) *AppError {
	return &AppError{
		Err:        sentinel,
		Message:    fmt.Sprintf(format, args...),
		StatusCode: statusCode,
	}
}

func HTTPStatusCode(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.StatusCode
	}

	switch {
	case errors.Is(err, ErrDocumentNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrDocumentExists), errors.Is(err, ErrIdempotencyConflict):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrShardUnavailable), errors.Is(err, ErrTimeout):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}

}
