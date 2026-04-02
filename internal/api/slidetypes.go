package api

import (
	"net/http"

	"github.com/sebahrens/json2pptx/internal/types"
)

// SlideTypesResponse is the response for GET /api/v1/slide-types.
type SlideTypesResponse struct {
	SlideTypes []types.SlideTypeInfo `json:"slide_types"`
}

// SlideTypesHandler returns a handler for GET /api/v1/slide-types.
// This endpoint returns all supported slide types with descriptions.
func SlideTypesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, SlideTypesResponse{
			SlideTypes: types.SupportedSlideTypes(),
		})
	}
}
