package handlers

import (
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/response"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	userapp "github.com/oksasatya/go-ddd-clean-architecture/internal/application"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
)

type UserHandler struct {
	Svc    *userapp.Service
	JWT    *helpers.JWTManager
	Logger *logrus.Logger

	CookieDomain string
	CookieSecure bool
}

func NewUserHandler(svc *userapp.Service, jwt *helpers.JWTManager, logger *logrus.Logger, cookieDomain string, cookieSecure bool) *UserHandler {
	return &UserHandler{Svc: svc, JWT: jwt, Logger: logger, CookieDomain: cookieDomain, CookieSecure: cookieSecure}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type updateProfileRequest struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

func (h *UserHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" || req.Password == "" {
		resp := response.Error[any](c, http.StatusBadRequest, "invalid payload", nil)
		c.JSON(resp.Status, resp)
		return
	}
	res, pair, err := h.Svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		resp := response.Error[any](c, http.StatusUnauthorized, "invalid credentials", nil)
		c.JSON(resp.Status, resp)
		return
	}
	setAuthCookies(c, pair.AccessToken, pair.AccessTokenExpiry, pair.RefreshToken, pair.RefreshTokenExpiry, h.CookieDomain, h.CookieSecure)
	resp := response.Success(c, http.StatusOK, res, "login successful", map[string]any{"access_expires_at": pair.AccessTokenExpiry, "refresh_expires_at": pair.RefreshTokenExpiry})
	c.JSON(resp.Status, resp)
}

func (h *UserHandler) Refresh(c *gin.Context) {
	refresh, err := c.Cookie("refresh_token")
	if err != nil || refresh == "" {
		resp := response.Error[any](c, http.StatusUnauthorized, "missing refresh token", nil)
		c.JSON(resp.Status, resp)
		return
	}
	pair, _, err := h.Svc.Refresh(c.Request.Context(), refresh)
	if err != nil {
		resp := response.Error[any](c, http.StatusUnauthorized, "invalid refresh token", nil)
		c.JSON(resp.Status, resp)
		return
	}
	setAuthCookies(c, pair.AccessToken, pair.AccessTokenExpiry, pair.RefreshToken, pair.RefreshTokenExpiry, h.CookieDomain, h.CookieSecure)
	resp := response.Success[any](c, http.StatusOK, map[string]any{"refreshed": true}, "token refreshed", map[string]any{"access_expires_at": pair.AccessTokenExpiry, "refresh_expires_at": pair.RefreshTokenExpiry})
	c.JSON(resp.Status, resp)
}

func (h *UserHandler) Logout(c *gin.Context) {
	clearAuthCookies(c, h.CookieDomain, h.CookieSecure)
	resp := response.Success[any](c, http.StatusOK, map[string]any{"logged_out": true}, "logged out", nil)
	c.JSON(resp.Status, resp)
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	uid := c.GetString("userID")
	u, err := h.Svc.GetProfile(uid)
	if err != nil {
		resp := response.Error[any](c, http.StatusNotFound, "user not found", nil)
		c.JSON(resp.Status, resp)
		return
	}
	resp := response.Success(c, http.StatusOK, gin.H{
		"id":         u.ID,
		"email":      u.Email,
		"name":       u.Name,
		"avatar_url": u.AvatarURL,
		"created_at": u.CreatedAt,
		"updated_at": u.UpdatedAt,
	}, "profile", nil)
	c.JSON(resp.Status, resp)
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	uid := c.GetString("userID")
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp := response.Error[any](c, http.StatusBadRequest, "invalid payload", nil)
		c.JSON(resp.Status, resp)
		return
	}
	u, err := h.Svc.UpdateProfile(c.Request.Context(), uid, userapp.UpdateProfileInput{Name: req.Name, AvatarURL: req.AvatarURL})
	if err != nil {
		resp := response.Error[any](c, http.StatusBadRequest, "failed to update profile", err.Error())
		c.JSON(resp.Status, resp)
		return
	}
	resp := response.Success(c, http.StatusOK, gin.H{
		"id":         u.ID,
		"email":      u.Email,
		"name":       u.Name,
		"avatar_url": u.AvatarURL,
		"created_at": u.CreatedAt,
		"updated_at": u.UpdatedAt,
	}, "profile updated", nil)
	c.JSON(resp.Status, resp)
}

func setAuthCookies(c *gin.Context, access string, aexp time.Time, refresh string, rexp time.Time, domain string, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	// MaxAge in seconds
	aMax := int(time.Until(aexp).Seconds())
	rMax := int(time.Until(rexp).Seconds())
	c.SetCookie("access_token", access, aMax, "/", domain, secure, true)
	c.SetCookie("refresh_token", refresh, rMax, "/", domain, secure, true)
}

func clearAuthCookies(c *gin.Context, domain string, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("access_token", "", -1, "/", domain, secure, true)
	c.SetCookie("refresh_token", "", -1, "/", domain, secure, true)
}
