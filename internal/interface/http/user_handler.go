package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	userapp "github.com/oksasatya/go-ddd-clean-architecture/internal/application"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/response"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/validation"
)

type UserHandler struct {
	Svc     *userapp.Service
	JWT     *helpers.JWTManager
	Logger  *logrus.Logger
	Cookies *helpers.Manager
}

func NewUserHandler(svc *userapp.Service, jwt *helpers.JWTManager, logger *logrus.Logger, cookieDomain string, cookieSecure bool) *UserHandler {
	return &UserHandler{Svc: svc, JWT: jwt, Logger: logger, Cookies: helpers.NewCookie(cookieDomain, cookieSecure)}
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,pwd"`
}

type updateProfileRequest struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

func (h *UserHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error[any](c, http.StatusBadRequest, "invalid payload", validation.ToDetails(err))
		return
	}

	res, pair, err := h.Svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.Error[any](c, http.StatusUnauthorized, "invalid credentials", nil)
		return
	}
	h.Cookies.SetPair(c, pair.AccessToken, pair.AccessTokenExpiry, pair.RefreshToken, pair.RefreshTokenExpiry)
	response.Success(c, http.StatusOK, res, "login successful", map[string]any{"access_expires_at": pair.AccessTokenExpiry, "refresh_expires_at": pair.RefreshTokenExpiry})
}

func (h *UserHandler) Refresh(c *gin.Context) {
	refresh, err := c.Cookie("refresh_token")
	if err != nil || refresh == "" {
		response.Error[any](c, http.StatusUnauthorized, "missing refresh token", nil)
		return
	}
	pair, _, err := h.Svc.Refresh(c.Request.Context(), refresh)
	if err != nil {
		response.Error[any](c, http.StatusUnauthorized, "invalid refresh token", nil)
		return
	}
	h.Cookies.SetPair(c, pair.AccessToken, pair.AccessTokenExpiry, pair.RefreshToken, pair.RefreshTokenExpiry)
	response.Success[any](c, http.StatusOK, map[string]any{"refreshed": true}, "token refreshed", map[string]any{"access_expires_at": pair.AccessTokenExpiry, "refresh_expires_at": pair.RefreshTokenExpiry})
}

func (h *UserHandler) Logout(c *gin.Context) {
	h.Cookies.Clear(c)
	response.Success[any](c, http.StatusOK, map[string]any{"logged_out": true}, "logged out", nil)
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	uid := c.GetString("userID")
	u, err := h.Svc.GetProfile(uid)
	if err != nil {
		response.Error[any](c, http.StatusNotFound, "user not found", nil)
		return
	}
	response.Success(c, http.StatusOK, gin.H{
		"id":         u.ID,
		"email":      u.Email,
		"name":       u.Name,
		"avatar_url": u.AvatarURL,
		"created_at": u.CreatedAt,
		"updated_at": u.UpdatedAt,
	}, "profile", nil)
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	uid := c.GetString("userID")
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error[any](c, http.StatusBadRequest, "invalid payload", validation.ToDetails(err))
		return
	}
	u, err := h.Svc.UpdateProfile(c.Request.Context(), uid, userapp.UpdateProfileInput{Name: req.Name, AvatarURL: req.AvatarURL})
	if err != nil {
		response.Error[any](c, http.StatusBadRequest, "failed to update profile", err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{
		"id":         u.ID,
		"email":      u.Email,
		"name":       u.Name,
		"avatar_url": u.AvatarURL,
		"created_at": u.CreatedAt,
		"updated_at": u.UpdatedAt,
	}, "profile updated", nil)
}
