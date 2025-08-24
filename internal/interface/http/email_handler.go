package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/oksasatya/go-ddd-clean-architecture/config"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/mailer"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/response"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/validation"
)

type EmailHandler struct {
	Pub    *helpers.RabbitPublisher
	Logger *logrus.Logger
	Cfg    *config.Config
}

func NewEmailHandler(pub *helpers.RabbitPublisher, logger *logrus.Logger, cfg *config.Config) *EmailHandler {
	return &EmailHandler{Pub: pub, Logger: logger, Cfg: cfg}
}

type sendEmailRequest struct {
	To       string         `json:"to" binding:"required,email"`
	Template string         `json:"template"` // optional: login_notification, verify_email, forgot_password, profile_updated
	Data     map[string]any `json:"data"`     // optional template data
	Subject  string         `json:"subject"`  // required if no template
	Text     string         `json:"text"`     // optional if html provided
	HTML     string         `json:"html"`     // optional if text provided
}

// Send enqueues an email job to RabbitMQ.
func (h *EmailHandler) Send(c *gin.Context) {
	var req sendEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error[any](c, http.StatusBadRequest, "invalid payload", validation.ToDetails(err))
		return
	}

	// Validate: either template provided, or raw subject with at least text/html
	if req.Template == "" {
		if req.Subject == "" || (req.Text == "" && req.HTML == "") {
			response.Error[any](c, http.StatusBadRequest, "either template or subject with text/html is required", nil)
			return
		}
	}

	// If sending disabled, short-circuit
	if h.Cfg != nil && !h.Cfg.MailSendEnabled {
		response.Success[any](c, http.StatusAccepted, map[string]any{"enqueued": false, "disabled": true}, "email sending disabled", nil)
		return
	}

	job := mailer.EmailJob{To: req.To}
	if req.Template != "" {
		job.Template = req.Template
		job.Data = req.Data
	} else {
		job.Subject = req.Subject
		job.Text = req.Text
		job.HTML = req.HTML
	}
	if err := h.Pub.PublishJSON(c.Request.Context(), job); err != nil {
		if h.Logger != nil {
			h.Logger.WithError(err).Warn("failed to publish email job")
		}
		response.Error[any](c, http.StatusInternalServerError, "failed to enqueue", nil)
		return
	}
	response.Success[any](c, http.StatusAccepted, map[string]any{"enqueued": true}, "email enqueued", nil)
}
