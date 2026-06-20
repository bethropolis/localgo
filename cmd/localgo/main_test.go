package main

import (
	"testing"
)

func TestVersionVars(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestBuildVars(t *testing.T) {
	if GitCommit == "" {
		t.Error("GitCommit should not be empty")
	}
	if BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}
}
