package main

import "fmt"

func runVersion() error {
	fmt.Printf("json2pptx %s (commit: %s, built: %s)\n", Version, CommitSHA, BuildTime)
	return nil
}
