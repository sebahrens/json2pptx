package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
)

func acquireWork(w http.ResponseWriter, sem chan struct{}) bool {
	if sem == nil {
		return true
	}
	select {
	case sem <- struct{}{}:
		return true
	default:
		w.Header().Set("Retry-After", "5")
		writeError(w, http.StatusServiceUnavailable, apierrors.CodeRateLimited, "Too many concurrent requests; retry later", nil)
		return false
	}
}

func writeWorkError(w http.ResponseWriter, ctx context.Context, err error, message string) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		writeError(w, http.StatusGatewayTimeout, apierrors.CodeRequestTimeout, "Request processing timed out", nil)
		return
	}
	slog.Error(message, "error", err)
	writeError(w, http.StatusInternalServerError, apierrors.CodeRenderError, message, nil)
}
