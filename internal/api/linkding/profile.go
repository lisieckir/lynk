package linkding

import (
	"errors"
	"net/http"

	"github.com/lisieckir/lynk/internal/user"
)

func (a *API) getProfile(w http.ResponseWriter, r *http.Request, userID int64) {
	profile, err := user.GetProfile(r.Context(), a.db, userID)
	switch {
	case errors.Is(err, user.ErrProfileNotFound):
		writeNotFound(w)
	case err != nil:
		writeInternalError(w, a.logger, "GET /api/user/profile/", err)
	default:
		writeJSON(w, http.StatusOK, newProfileDTO(profile))
	}
}
