// Package pptx provides PPTX file manipulation primitives.
package pptx

import "os"

// osFileWrapper wraps *os.File to implement readerAtFile interface.
type osFileWrapper struct {
	*os.File
}

func (w *osFileWrapper) Stat() (fileInfo, error) {
	return w.File.Stat()
}

// openOSFile opens a file and returns it wrapped for the readerAtFile interface.
func openOSFile(path string) (readerAtFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &osFileWrapper{f}, nil
}
