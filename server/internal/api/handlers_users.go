package api

import (
	"encoding/json"
	"net/http"

	"github.com/southcorner/systemcheck/server/internal/auth"
	"github.com/southcorner/systemcheck/server/internal/model"
)

func (a *App) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.Store.ListAdmins(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query error")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

type createUserReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (a *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "email and password required")
		return
	}
	role := model.Role(req.Role)
	if !model.ValidRole(role) {
		writeErr(w, http.StatusBadRequest, "invalid role")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash error")
		return
	}
	id, err := a.Store.CreateAdmin(r.Context(), req.Email, hash, role)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "could not create user (duplicate email?)")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "create_user", req.Email,
		map[string]interface{}{"role": req.Role})
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

type roleReq struct {
	Role string `json:"role"`
}

func (a *App) handleSetUserRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req roleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	role := model.Role(req.Role)
	if !model.ValidRole(role) {
		writeErr(w, http.StatusBadRequest, "invalid role")
		return
	}
	// Guard: do not demote the last enabled admin.
	if role != model.RoleAdmin {
		target, err := a.Store.GetAdminByID(r.Context(), id)
		if err == nil && target.Role == model.RoleAdmin {
			if n, _ := a.Store.CountEnabledAdmins(r.Context()); n <= 1 {
				writeErr(w, http.StatusBadRequest, "cannot demote the last admin")
				return
			}
		}
	}
	if err := a.Store.SetRole(r.Context(), id, role); err != nil {
		writeErr(w, http.StatusInternalServerError, "update error")
		return
	}
	_ = a.Store.Audit(r.Context(), currentUser(r).Email, "set_user_role", id,
		map[string]interface{}{"role": req.Role})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type disableReq struct {
	Disabled bool `json:"disabled"`
}

func (a *App) handleDisableUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req disableReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	self := currentUser(r)
	if req.Disabled && id == self.ID {
		writeErr(w, http.StatusBadRequest, "cannot disable yourself")
		return
	}
	// Guard: do not disable the last enabled admin.
	if req.Disabled {
		target, err := a.Store.GetAdminByID(r.Context(), id)
		if err == nil && target.Role == model.RoleAdmin && !target.Disabled {
			if n, _ := a.Store.CountEnabledAdmins(r.Context()); n <= 1 {
				writeErr(w, http.StatusBadRequest, "cannot disable the last admin")
				return
			}
		}
	}
	if err := a.Store.SetDisabled(r.Context(), id, req.Disabled); err != nil {
		writeErr(w, http.StatusInternalServerError, "update error")
		return
	}
	_ = a.Store.Audit(r.Context(), self.Email, "set_user_disabled", id,
		map[string]interface{}{"disabled": req.Disabled})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
