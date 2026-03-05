// handlers/auth_handler.go — HTTP only: parse → call AuthService → respond
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"pokemontool/pkg"
	"pokemontool/services"
)

type AuthHandler struct{ svc *services.AuthService }

func NewAuthHandler(svc *services.AuthService) *AuthHandler { return &AuthHandler{svc: svc} }

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		pkg.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, token, err := h.svc.Register(r.Context(), body.Email, body.Password, body.DisplayName)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, services.ErrEmailTaken) { status = http.StatusConflict }
		if errors.Is(err, services.ErrWeakPassword) { status = http.StatusBadRequest }
		pkg.Error(w, status, err.Error())
		return
	}
	pkg.JSON(w, http.StatusCreated, map[string]interface{}{"token": token, "user": user})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		pkg.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, token, err := h.svc.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		pkg.Error(w, http.StatusUnauthorized, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{"token": token, "user": user})
}
