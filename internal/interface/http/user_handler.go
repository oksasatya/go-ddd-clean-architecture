package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oksasatya/go-ddd-clean-architecture/config"
	tpl "github.com/oksasatya/go-ddd-clean-architecture/pkg/mailer/templates"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	userapp "github.com/oksasatya/go-ddd-clean-architecture/internal/application"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/mailer"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/response"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/validation"
)

type UserHandler struct {
	Svc     *userapp.Service
	JWT     *helpers.JWTManager
	Logger  *logrus.Logger
	Cookies *helpers.Manager
	Pub     *helpers.RabbitPublisher
	Cfg     *config.Config
	RDB     *redis.Client
}

func NewUserHandler(svc *userapp.Service, jwt *helpers.JWTManager, logger *logrus.Logger, cookieDomain string, cookieSecure bool, pub *helpers.RabbitPublisher, cfg *config.Config, rdb *redis.Client) *UserHandler {
	return &UserHandler{Svc: svc, JWT: jwt, Logger: logger, Cookies: helpers.NewCookie(cookieDomain, cookieSecure), Pub: pub, Cfg: cfg, RDB: rdb}
}

type loginRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,pwd"`
}

type updateProfileRequest struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// setTokenCookies centralizes auth cookie setting to avoid duplication
func (h *UserHandler) setTokenCookies(c *gin.Context, pair userapp.TokenPair) {
	h.Cookies.SetPair(c, pair.AccessToken, pair.AccessTokenExpiry, pair.RefreshToken, pair.RefreshTokenExpiry)
}

func (h *UserHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error[any](c, http.StatusBadRequest, "invalid payload", validation.ToDetails(err))
		return
	}

	u, pair, err := h.Svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, userapp.ErrEmailNotVerified) {
			response.Success[any](c, http.StatusAccepted, map[string]any{
				"requires_verification": true,
			}, "email not verified", nil)
			return
		}
		status := http.StatusUnauthorized
		msg := "invalid credentials"
		// For non-auth errors, respond 500
		if !errors.Is(err, userapp.ErrInvalidCredentials) {
			status = http.StatusInternalServerError
			msg = "login failed"
		}
		response.Error[any](c, status, msg, nil)
		return
	}

	// Check trusted device (30 days)
	deviceID, _ := c.Cookie("device_id")
	trusted := false
	if deviceID != "" && h.RDB != nil {
		if v, _ := h.RDB.Get(c, helpers.KeyTrustedDevice(u.UserID, deviceID)).Result(); v == "1" {
			trusted = true
		}
	}

	if trusted {
		// Set cookies using the pair returned from Login
		h.setTokenCookies(c, pair)
		payload := map[string]any{
			"user_id": u.UserID,
			"email":   u.Email,
			"name":    u.Name,
		}
		response.Success(c, http.StatusOK, payload, "login successful", map[string]any{"access_expires_at": pair.AccessTokenExpiry, "refresh_expires_at": pair.RefreshTokenExpiry})
		return
	}

	// Not trusted: generate OTP, store for 10 minutes, send email
	if h.RDB == nil || h.Pub == nil {
		response.Error[any](c, http.StatusServiceUnavailable, "otp unavailable", nil)
		return
	}
	code, err := helpers.GenOTPCode()
	if err != nil {
		response.Error[any](c, http.StatusInternalServerError, "otp generation failed", nil)
		return
	}
	_ = h.RDB.Set(c, helpers.KeyLoginOTP(u.UserID), code, 10*time.Minute).Err()

	ip := c.GetString("real_ip")
	if ip == "" {
		ip = c.ClientIP()
	}
	ua := c.GetHeader("User-Agent")
	resolver := tpl.IPAPIResolver{}
	data := tpl.NewLoginOTPData(
		h.Cfg,
		u.Name,
		u.Email,
		code,
		tpl.WithTime(time.Now()),
		tpl.WithExpiresIn(10*time.Minute),
		tpl.WithIP(ip),
		tpl.WithUserAgent(ua),
		tpl.WithGeoFromIP(c.Request.Context(), resolver, ip),
	)
	job := mailer.EmailJob{To: u.Email, Template: "universal", Data: data}
	if h.Cfg != nil && h.Cfg.MailSendEnabled && h.Pub != nil {
		go func(job mailer.EmailJob) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = h.Pub.PublishJSON(ctx, job)
		}(job)
	}

	response.Success[any](c, http.StatusAccepted, map[string]any{
		"requires_otp": true,
	}, "otp required", nil)
}

// LoginOTPConfirm - POST /api/login/otp/confirm {email, code, remember_device}
func (h *UserHandler) LoginOTPConfirm(c *gin.Context) {
	var req struct {
		Email          string `json:"email" binding:"required,email"`
		Code           string `json:"code" binding:"required"`
		RememberDevice bool   `json:"remember_device"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error[any](c, http.StatusBadRequest, "invalid payload", validation.ToDetails(err))
		return
	}
	if h.RDB == nil {
		response.Error[any](c, http.StatusServiceUnavailable, "otp unavailable", nil)
		return
	}
	// Normalize and validate OTP format (6 digits)
	req.Code = strings.TrimSpace(req.Code)
	if ok, _ := regexp.MatchString(`^[0-9]{6}$`, req.Code); !ok {
		response.Error[any](c, http.StatusUnauthorized, "invalid or expired code", nil)
		return
	}

	u, err := h.Svc.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil || u == nil {
		response.Error[any](c, http.StatusUnauthorized, "invalid code", nil)
		return
	}

	stored, err := h.RDB.Get(c, helpers.KeyLoginOTP(u.ID)).Result()
	if err != nil || stored == "" {
		response.Error[any](c, http.StatusUnauthorized, "invalid or expired code", nil)
		return
	}
	if stored != req.Code {
		response.Error[any](c, http.StatusUnauthorized, "invalid or expired code", nil)
		return
	}
	// Consume OTP
	_ = h.RDB.Del(c, helpers.KeyLoginOTP(u.ID)).Err()

	pair, err := h.Svc.IssueTokens(c.Request.Context(), u)
	if err != nil {
		response.Error[any](c, http.StatusInternalServerError, "login failed", nil)
		return
	}

	// Remember device if requested
	if req.RememberDevice {
		// generate a device id and set trusted for 30 days
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err == nil {
			devID := base64.RawURLEncoding.EncodeToString(buf)
			exp := time.Now().Add(30 * 24 * time.Hour)
			_ = h.RDB.Set(c, helpers.KeyTrustedDevice(u.ID, devID), "1", 30*24*time.Hour).Err()
			h.Cookies.SetDeviceID(c, devID, exp)
		}
	}

	h.setTokenCookies(c, pair)
	payload := map[string]any{
		"user_id": u.ID,
		"email":   u.Email,
		"name":    u.Name,
	}
	response.Success(c, http.StatusOK, payload, "login successful", map[string]any{"access_expires_at": pair.AccessTokenExpiry, "refresh_expires_at": pair.RefreshTokenExpiry})
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
	h.setTokenCookies(c, pair)
	response.Success[any](c, http.StatusOK, map[string]any{"refreshed": true}, "token refreshed", map[string]any{"access_expires_at": pair.AccessTokenExpiry, "refresh_expires_at": pair.RefreshTokenExpiry})
}

func (h *UserHandler) Logout(c *gin.Context) {
	// Clear only auth cookies; keep device_id so trusted device remains for 30 days
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("access_token", "", -1, "/", h.Cookies.Domain, h.Cookies.Secure, true)
	c.SetCookie("refresh_token", "", -1, "/", h.Cookies.Domain, h.Cookies.Secure, true)
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

	before, _ := h.Svc.GetProfile(uid)

	u, err := h.Svc.UpdateProfile(
		c.Request.Context(),
		uid,
		userapp.UpdateProfileInput{
			Name:      req.Name,
			AvatarURL: req.AvatarURL,
		},
	)
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

	if h.Pub != nil && before != nil {
		changes := map[string]string{}

		if u.Name != "" && u.Name != before.Name {
			changes["name"] = u.Name
		}
		if u.AvatarURL != "" && u.AvatarURL != before.AvatarURL {
			changes["avatar_url"] = u.AvatarURL
		}

		if len(changes) == 0 {
			return
		}

		data := tpl.NewProfileUpdatedData(
			h.Cfg,
			u.Name,  // name
			u.Email, // email
			changes,
			tpl.WithTime(time.Now()),
		)

		job := mailer.EmailJob{
			To:       u.Email,
			Template: "universal",
			Data:     data,
		}

		if h.Cfg != nil && h.Cfg.MailSendEnabled {
			go func(job mailer.EmailJob) {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if err := h.Pub.PublishJSON(ctx, job); err != nil && h.Logger != nil {
					h.Logger.WithError(err).Warn("failed to enqueue profile updated email")
				}
			}(job)
		}
	}
}

// Search allows searching users via Elasticsearch.
func (h *UserHandler) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		response.Error[any](c, http.StatusBadRequest, "missing q", nil)
		return
	}
	size := 10
	if s := c.Query("size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			size = v
		}
	}
	res, err := h.Svc.SearchUsers(c.Request.Context(), q, size)
	if err != nil {
		response.Error[any](c, http.StatusInternalServerError, "search failed", err.Error())
		return
	}
	response.Success[any](c, http.StatusOK, res, "search results", nil)
}
