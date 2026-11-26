package linkding

import "net/http"

// listTags serves GET /api/tags/ with the same LimitOffsetPagination contract
// as the bookmark lists: limit/offset bound the page, count is the total
// number of owned tags, and next/previous are request-derived URLs that are
// null at the ends of the result set. The extension requests limit=5000 to
// load the whole tag set in one call.
func (a *API) listTags(w http.ResponseWriter, r *http.Request, userID int64) {
	q := r.URL.Query()
	page, err := a.service.ListTags(r.Context(), userID,
		parseLimitParam(q.Get("limit")), parseOffsetParam(q.Get("offset")))
	if err != nil {
		writeInternalError(w, a.logger, "GET /api/tags/", err)
		return
	}
	results := make([]tagDTO, 0, len(page.Items))
	for _, item := range page.Items {
		results = append(results, newTagDTO(item))
	}
	next, previous := pageLinks(r, page.Count, page.Limit, page.Offset)
	writeJSON(w, http.StatusOK, tagPageDTO{
		Count:    page.Count,
		Next:     next,
		Previous: previous,
		Results:  results,
	})
}
