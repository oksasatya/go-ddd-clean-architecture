package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/oksasatya/go-ddd-clean-architecture/config"
	repo "github.com/oksasatya/go-ddd-clean-architecture/internal/domain/repository"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/mailer"
	tpl "github.com/oksasatya/go-ddd-clean-architecture/pkg/mailer/templates"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/response"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/validation"

	// added for sqlc-based audit logging
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/infrastructure/postgres/pgstore"
)

type AuthHandler struct {
	Repo   repo.UserRepository
	RDB    *redis.Client
	Logger *logrus.Logger
	Cfg    *config.Config
	Pub    *helpers.RabbitPublisher
	DB     *pgxpool.Pool
}

func NewAuthHandler(repo repo.UserRepository, rdb *redis.Client, logger *logrus.Logger, cfg *config.Config, pub *helpers.RabbitPublisher, db *pgxpool.Pool) *AuthHandler {
	return &AuthHandler{Repo: repo, RDB: rdb, Logger: logger, Cfg: cfg, Pub: pub, DB: db}
}

// Key helpers
func keyVerifyToken(t string) string { return "email:verify:token:" + t }
func keyResetToken(t string) string  { return "pwd:reset:token:" + t }
func keyVerified(uid string) string  { return "user:verified:" + uid }

func clientIP(c *gin.Context) string {
	if ip := c.GetString("real_ip"); ip != "" {
		return ip
	}
	return c.ClientIP()
}

func (h *AuthHandler) genToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *AuthHandler) audit(c *gin.Context, userID string, email string, action string, metadata map[string]any) {
	if h.DB == nil {
		return
	}
	md, _ := json.Marshal(metadata)
	ip := clientIP(c)
	ua := c.GetHeader("User-Agent")

	q := pgstore.New(h.DB)

	var uid pgtype.UUID
	if userID != "" {
		if parsed, err := uuid.Parse(userID); err == nil {
			uid.Bytes = parsed
			uid.Valid = true
		}
	}
	var emailTxt pgtype.Text
	if email != "" {
		emailTxt.String = email
		emailTxt.Valid = true
	}
	var ipTxt pgtype.Text
	if ip != "" {
		ipTxt.String = ip
		ipTxt.Valid = true
	}
	var uaTxt pgtype.Text
	if ua != "" {
		uaTxt.String = ua
		uaTxt.Valid = true
	}
	_ = q.InsertAuditLog(c, pgstore.InsertAuditLogParams{
		UserID:    uid,
		Email:     emailTxt,
		Action:    action,
		Ip:        ipTxt,
		UserAgent: uaTxt,
		Metadata:  md,
	})
}

// VerifyInit POST /api/auth/verify/init (auth required)
// Returns a verification link that embeds the token in the front-end URL
func (h *AuthHandler) VerifyInit(c *gin.Context) {
	uid := c.GetString("userID")
	if uid == "" {
		response.Error[any](c, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	// If already verified in DB or Redis, return idempotent OK
	if ok, err := h.Repo.IsVerified(uid); err == nil && ok {
		if h.RDB != nil {
			_ = h.RDB.Set(c, keyVerified(uid), "1", 0).Err()
		}
		h.audit(c, uid, "", "verify_init_already", nil)
		response.Success(c, http.StatusOK, gin.H{"already_verified": true}, "already verified", nil)
		return
	}
	if h.RDB != nil {
		if v, _ := h.RDB.Get(c, keyVerified(uid)).Result(); v == "1" {
			h.audit(c, uid, "", "verify_init_already", map[string]any{"source": "redis"})
			response.Success(c, http.StatusOK, gin.H{"already_verified": true}, "already verified", nil)
			return
		}
	}
	// Create token and store mapping -> uid
	tok, err := h.genToken(32)
	if err != nil {
		response.Error[any](c, http.StatusInternalServerError, "token generation failed", nil)
		return
	}
	if h.RDB != nil {
		h.RDB.Set(c, keyVerifyToken(tok), uid, 24*time.Hour)
	}
	link := h.Cfg.VerifyEmailURL + "?token=" + tok
	h.audit(c, uid, "", "verify_init_issue", map[string]any{"link": link})

	// enqueue verify email
	if h.Pub != nil && h.Cfg != nil && h.Cfg.MailSendEnabled {
		u, _ := h.Repo.GetByID(uid)
		if u != nil {
			ip := clientIP(c)
			ua := c.GetHeader("User-Agent")
			resolver := tpl.IPAPIResolver{}
			data := tpl.NewVerifyEmailData(
				h.Cfg,
				u.Name,
				u.Email,
				link,
				tpl.WithTime(time.Now()),
				tpl.WithExpiresIn(24*time.Hour),
				tpl.WithIP(ip),
				tpl.WithUserAgent(ua),
				tpl.WithGeoFromIP(c.Request.Context(), resolver, ip),
			)
			job := mailer.EmailJob{To: u.Email, Template: "universal", Data: data}
			_ = h.Pub.PublishJSON(c, job)
		}
	}

	response.Success(c, http.StatusOK, gin.H{"verify_link": link}, "verification link", nil)
}

// VerifyConfirm POST /api/auth/verify/confirm {token}
func (h *AuthHandler) VerifyConfirm(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error[any](c, http.StatusBadRequest, "invalid payload", validation.ToDetails(err))
		return
	}
	if h.RDB == nil {
		response.Error[any](c, http.StatusInternalServerError, "verification unavailable", nil)
		return
	}
	uid, err := h.RDB.Get(c, keyVerifyToken(req.Token)).Result()
	if err != nil || uid == "" {
		response.Error[any](c, http.StatusBadRequest, "invalid or expired token", nil)
		return
	}
	// Mark verified in DB and cache
	_ = h.Repo.SetVerified(uid)
	h.RDB.Set(c, keyVerified(uid), "1", 0)
	h.RDB.Del(c, keyVerifyToken(req.Token))
	h.audit(c, uid, "", "verify_confirm", map[string]any{"token": "redacted"})
	response.Success[any](c, http.StatusOK, gin.H{"verified": true}, "email verified", nil)
}

// ResetInit - POST /api/auth/reset/init {email}
// Returns a reset link that embeds the token in the front-end URL
func (h *AuthHandler) ResetInit(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error[any](c, http.StatusBadRequest, "invalid payload", validation.ToDetails(err))
		return
	}
	// Always return OK to avoid enumeration
	link := ""
	u, _ := h.Repo.GetByEmail(req.Email)
	if u != nil && h.RDB != nil {
		tok, err := h.genToken(32)
		if err != nil {
			response.Error[any](c, http.StatusInternalServerError, "token generation failed", nil)
			return
		}
		h.RDB.Set(c, keyResetToken(tok), u.ID, 30*time.Minute)
		link = h.Cfg.ResetPasswordURL + "?token=" + tok
		// enqueue email
		if h.Pub != nil && h.Cfg != nil && h.Cfg.MailSendEnabled {
			ip := clientIP(c)
			ua := c.GetHeader("User-Agent")
			resolver := tpl.IPAPIResolver{}
			data := tpl.NewForgotPasswordData(
				h.Cfg,
				u.Name,
				u.Email,
				u.Email,
				tpl.WithTime(time.Now()),
				tpl.WithResetURL(link),
				tpl.WithExpiresIn(30*time.Minute),
				tpl.WithIP(ip),
				tpl.WithUserAgent(ua),
				tpl.WithGeoFromIP(c.Request.Context(), resolver, ip),
			)
			job := mailer.EmailJob{To: u.Email, Template: "universal", Data: data}
			_ = h.Pub.PublishJSON(c, job)
		}
		h.audit(c, u.ID, u.Email, "reset_init_issue", map[string]any{"link": link})
	} else {
		// log attempted reset with unknown email
		h.audit(c, "", req.Email, "reset_init_unknown", nil)
	}
	response.Success(c, http.StatusOK, gin.H{"reset_link": link}, "reset link", nil)
}

// POST /api/auth/reset/confirm {token, new_password}
func (h *AuthHandler) ResetConfirm(c *gin.Context) {
	var req struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,pwd"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error[any](c, http.StatusBadRequest, "invalid payload", validation.ToDetails(err))
		return
	}
	if h.RDB == nil {
		response.Error[any](c, http.StatusInternalServerError, "reset unavailable", nil)
		return
	}
	uid, err := h.RDB.Get(c, keyResetToken(req.Token)).Result()
	if err != nil || uid == "" {
		response.Error[any](c, http.StatusBadRequest, "invalid or expired token", nil)
		return
	}
	hash, err := helpers.HashPassword(req.NewPassword)
	if err != nil {
		response.Error[any](c, http.StatusInternalServerError, "hash fail", nil)
		return
	}
	if err := h.Repo.UpdatePassword(uid, hash); err != nil {
		response.Error[any](c, http.StatusInternalServerError, "update fail", nil)
		return
	}
	h.RDB.Del(c, keyResetToken(req.Token))
	h.audit(c, uid, "", "reset_confirm", map[string]any{"token": "redacted"})
	response.Success[any](c, http.StatusOK, gin.H{"reset": true}, "password updated", nil)
}
