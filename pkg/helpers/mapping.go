package helpers

import (
	"fmt"
	"strings"

	"github.com/oksasatya/go-ddd-clean-architecture/pkg/mailer"
	mailtpl "github.com/oksasatya/go-ddd-clean-architecture/pkg/mailer/templates"
)

func SubjectForUniversal(data map[string]any) string {
	typeStr := fmt.Sprintf("%v", data["Type"])
	switch strings.ToLower(typeStr) {
	case mailtpl.LoginNotification:
		return "New login to your account"
	case mailtpl.VerifyEmail:
		return "Verify your email address"
	case mailtpl.ForgotPassword:
		return "Reset your password"
	case mailtpl.ProfileUpdated:
		return "Your profile was updated successfully"
	case mailtpl.LoginOTP:
		return "Your login verification code"
	default:
		return "Notification"
	}
}

func EnsureRecipientAndEmail(job *mailer.EmailJob) {
	if job.Data == nil {
		job.Data = map[string]any{}
	}
	if v, ok := job.Data["Email"]; !ok || fmt.Sprintf("%v", v) == "" {
		job.Data["Email"] = job.To
	}
	if v, ok := job.Data["RecipientEmail"]; !ok || fmt.Sprintf("%v", v) == "" {
		job.Data["RecipientEmail"] = job.To
	}
}

func MapLegacyToUniversal(job *mailer.EmailJob) {
	switch strings.ToLower(job.Template) {
	case "login_notification", "verify_email", "forgot_password", "profile_updated", "login_otp":
		if job.Data == nil {
			job.Data = map[string]any{}
		}
		if _, ok := job.Data["Type"]; !ok || fmt.Sprintf("%v", job.Data["Type"]) == "" {
			job.Data["Type"] = job.Template
		}
		job.Template = "universal"
	}
}
