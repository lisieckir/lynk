package linkding

import "net/http"

func (a *API) listTags(w http.ResponseWriter, r *http.Request, userID int64) {
	items, err := a.service.ListTags(r.Context(), userID)
	if err != nil {
		writeInternalError(w, a.logger, "GET /api/tags/", err)
		return
	}
	results := make([]tagDTO, 0, len(items))
	for _, item := range items {
		results = append(results, newTagDTO(item))
	}
	writeJSON(w, http.StatusOK, tagPageDTO{Count: len(results), Results: results})
}
