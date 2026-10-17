package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
)

// SemanticKindsProvider delegates catalogue construction to the shared
// list_slide_kinds builder. Valid is false for a rejected query.
type SemanticKindsProvider func(context.Context, []string, []string, string) (result json.RawMessage, valid bool, err error)

func SemanticKindsHandler(provider SemanticKindsProvider, sem chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if provider == nil {
			writeError(w, http.StatusNotImplemented, apierrors.CodeUnsupportedFeature, "Slide-kind discovery is not configured", nil)
			return
		}
		templateName := strings.TrimSpace(r.URL.Query().Get("template"))
		if templateName != "" {
			if err := ValidateTemplateName(templateName); err != nil {
				writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest, "Invalid template name", nil)
				return
			}
		}
		if !acquireWork(w, sem) {
			return
		}
		if sem != nil {
			defer func() { <-sem }()
		}
		ctx, cancel := context.WithTimeout(r.Context(), DefaultConvertTimeout)
		defer cancel()
		split := func(s string) []string {
			if strings.TrimSpace(s) == "" {
				return nil
			}
			out := strings.Split(s, ",")
			for i := range out {
				out[i] = strings.TrimSpace(out[i])
			}
			return out
		}
		body, valid, err := provider(ctx, split(r.URL.Query().Get("kinds")), split(r.URL.Query().Get("fields")), templateName)
		if err != nil {
			writeWorkError(w, ctx, err, "Slide-kind discovery failed")
			return
		}
		status := http.StatusOK
		if !valid {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, body)
	}
}
