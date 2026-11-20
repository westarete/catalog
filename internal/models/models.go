// Package models is the daemon's connection to a language model. A Provider
// answers one request; Classify says whether a failure is the file's fault or
// the service's, which decides whether retrying later can help.
package models

import (
	"context"
	"errors"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
)

// Request is one question for the model. PDF, when set, is sent as a document
// alongside Text.
type Request struct {
	System, Text string
	PDF          []byte
	MaxTokens    int64
}

// Response is the model's reply and what it cost.
type Response struct {
	Text                      string
	InputTokens, OutputTokens int64
}

// Provider answers one Request.
type Provider interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

// Kind says who a failure belongs to.
type Kind int

const (
	// FileProblem means this file will keep failing; the service is fine.
	FileProblem Kind = iota
	// ServiceProblem means the service or network failed; the file may be fine.
	ServiceProblem
)

// ErrEmptyReply marks a reply with no usable text, such as a refusal or a
// reply cut off before any text.
var ErrEmptyReply = errors.New("model returned no profile text")

// Classify sorts an error into a FileProblem or a ServiceProblem. The API
// rejects a file with 400 (bad request) or 413 (too large); every other
// failure is treated as the service's.
func Classify(err error) Kind {
	if errors.Is(err, ErrEmptyReply) {
		return FileProblem
	}
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) &&
		(apiErr.StatusCode == http.StatusBadRequest || apiErr.StatusCode == http.StatusRequestEntityTooLarge) {
		return FileProblem
	}
	return ServiceProblem
}
