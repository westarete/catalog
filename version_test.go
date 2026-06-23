package main

import (
	"bytes"
	"fmt"
	"testing"
)

func TestVersionDefault(t *testing.T) {
	if version != "dev" {
		t.Errorf("default version = %q, want %q", version, "dev")
	}
}

func TestVersionOutput(t *testing.T) {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, "catalog", version)
	got := buf.String()
	want := "catalog dev\n"
	if got != want {
		t.Errorf("version output = %q, want %q", got, want)
	}
}
