package models

import (
	"errors"
	"fmt"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want Kind
	}{
		{"bad request", &anthropic.Error{StatusCode: 400}, FileProblem},
		{"too large", &anthropic.Error{StatusCode: 413}, FileProblem},
		{"unauthorized", &anthropic.Error{StatusCode: 401}, ServiceProblem},
		{"rate limited", &anthropic.Error{StatusCode: 429}, ServiceProblem},
		{"overloaded", &anthropic.Error{StatusCode: 529}, ServiceProblem},
		{"wrapped bad request", fmt.Errorf("call: %w", &anthropic.Error{StatusCode: 400}), FileProblem},
		{"network", errors.New("dial tcp"), ServiceProblem},
		{"empty reply", ErrEmptyReply, FileProblem},
		{"wrapped empty reply", fmt.Errorf("x: %w", ErrEmptyReply), FileProblem},
	}
	for _, c := range cases {
		if got := Classify(c.err); got != c.want {
			t.Errorf("%s: Classify = %v, want %v", c.name, got, c.want)
		}
	}
}
