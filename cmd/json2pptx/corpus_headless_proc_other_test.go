//go:build integration && !unix

package main

import "os/exec"

// CommandContext still bounds the direct process on non-Unix platforms.
func configureHeadlessProcess(cmd *exec.Cmd) {}
