package generator

import (
	"archive/zip"
	"errors"
	"strings"
	"testing"
)

type failingSlideWriter struct{ err error }

func (w failingSlideWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteSingleSlideRejectsInvalidHyperlinks(t *testing.T) {
	for _, tc := range []struct {
		name string
		link LinkSpec
	}{
		{"unsafe scheme", LinkSpec{URL: "javascript:alert(1)"}},
		{"missing target", LinkSpec{}},
		{"out of range slide", LinkSpec{Slide: 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newSinglePassContext("unused", nil, nil, false, nil)
			ctx.slideContentMap[1] = SlideSpec{SourceNote: "Source", SourceLink: &tc.link}
			err := ctx.writeSingleSlide(1, &slideXML{})
			if err == nil || !strings.Contains(err.Error(), "slide 1 hyperlink source") {
				t.Fatalf("writeSingleSlide accepted invalid link or lost context: %v", err)
			}
		})
	}
}

func TestWriteSingleSlidePropagatesArchiveFailure(t *testing.T) {
	failure := errors.New("output device unavailable")
	ctx := newSinglePassContext("unused", nil, nil, false, nil)
	ctx.outputWriter = zip.NewWriter(failingSlideWriter{failure})
	// Flush the archive's buffered writer to surface the device error before
	// creating the slide; subsequent Create must preserve that failure.
	if err := ctx.outputWriter.Flush(); err != nil {
		t.Fatal(err)
	}
	_, err := ctx.outputWriter.Create("seed")
	if err != nil {
		t.Fatal(err)
	}
	if err := ctx.outputWriter.Flush(); !errors.Is(err, failure) {
		t.Fatalf("seed flush: %v", err)
	}
	err = ctx.writeSingleSlide(1, &slideXML{})
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "ppt/slides/slide1.xml") {
		t.Fatalf("archive error lost: %v", err)
	}
}
