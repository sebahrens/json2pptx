package api

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
	"github.com/sebahrens/json2pptx/internal/render"
)

// ThumbnailsHandler renders only server-generated download artifacts. It never
// accepts arbitrary local paths or URLs, and shares the conversion semaphore.
func ThumbnailsHandler(outputDir string, sem chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filename := r.PathValue("filename")
		if !validDownloadFilename.MatchString(filename) {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest, "Invalid filename", nil)
			return
		}
		path := filepath.Join(outputDir, filename)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			writeError(w, http.StatusNotFound, apierrors.CodeFileNotFound, "File not found", nil)
			return
		}
		density, ok := boundedQueryInt(w, r, "density", 72, 36, 150)
		if !ok {
			return
		}
		maxSlides, ok := boundedQueryInt(w, r, "max_slides", 20, 1, 50)
		if !ok {
			return
		}
		indices, ok := thumbnailIndices(w, r)
		if !ok {
			return
		}
		if !acquireWork(w, sem) {
			return
		}
		if sem != nil {
			defer func() { <-sem }()
		}
		ctx, cancel := context.WithTimeout(r.Context(), DefaultConvertTimeout)
		defer cancel()
		var deck *render.DeckResult
		if len(indices) > 0 {
			deck, err = render.RenderDeckIndicesContext(ctx, path, density, indices, false)
		} else {
			deck, err = render.RenderDeckOptsContext(ctx, path, density, maxSlides, false)
		}
		if err != nil {
			var indexErr *render.IndexRangeError
			if errors.As(err, &indexErr) {
				writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest, "slide_indices contains an index outside the deck", nil)
				return
			}
			writeWorkError(w, ctx, err, "Thumbnail rendering failed")
			return
		}
		for i := range deck.Slides {
			slide := &deck.Slides[i]
			if slide.Path != "" {
				data, err := os.ReadFile(slide.Path)
				if err != nil {
					writeWorkError(w, ctx, err, "Thumbnail image unavailable")
					return
				}
				slide.PNG64 = base64.StdEncoding.EncodeToString(data)
			}
			slide.Path = ""
			slide.Cleanup = ""
		}
		writeJSON(w, http.StatusOK, deck)
	}
}

func boundedQueryInt(w http.ResponseWriter, r *http.Request, name string, fallback, min, max int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest, "Invalid "+name, map[string]any{"min": min, "max": max})
		return 0, false
	}
	return value, true
}

func thumbnailIndices(w http.ResponseWriter, r *http.Request) ([]int, bool) {
	var indices []int
	if raw := r.URL.Query().Get("slide_indices"); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) > 50 {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest, "At most 50 slide_indices are allowed", nil)
			return nil, false
		}
		for _, part := range parts {
			i, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || i < 0 {
				writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest, "slide_indices must contain nonnegative integers", nil)
				return nil, false
			}
			indices = append(indices, i)
		}
	}

	return indices, true
}
