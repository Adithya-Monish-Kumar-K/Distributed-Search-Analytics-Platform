package validator

import (
	"fmt"
	"strings"

	"github.com/Adithya-Monish-Kumar-K/searchplatform/internal/ingestion"
)

const (
	maxTitleLength = 1024
	maxBodyLength  = 1048576
	minBodyLength  = 1
)

type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	var parts []string
	for field, msg := range e.Fields {
		parts = append(parts, fmt.Sprintf("%s:%s", field, msg))
	}
	return strings.Join(parts, "; ")
}

func ValidateIngestRequest(req *ingestion.IngestRequest) error {
	errs := make(map[string]string)

	title := strings.TrimSpace(req.Title)
	if title == "" {
		errs["title"] = "title is required"
	} else if len(title) > maxTitleLength {
		errs["title"] = fmt.Sprintf("title must be at most %d characters", maxTitleLength)
	}
	body := strings.TrimSpace(req.Body)
	if len(body) < minBodyLength {
		errs["body"] = "Body is requred and must not be empty"
	} else if len(body) > maxBodyLength {
		errs["body"] = fmt.Sprintf("body must be at most %d characters", maxBodyLength)
	}
	if req.IdempotencyKey != "" && len(req.IdempotencyKey) > 255 {
		errs["idempotency_key"] = "idempotency key must be at most 255 characters"
	}
	if len(errs) > 0 {
		return &ValidationError{Fields: errs}
	}
	return nil
}
