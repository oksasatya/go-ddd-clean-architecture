package mailer

// EmailJob is the JSON payload put on the RabbitMQ queue for sending email.
// Html is optional; Text is recommended as fallback.
// You can also use a template by specifying Template and Data.
type EmailJob struct {
	To       string         `json:"to"`
	Subject  string         `json:"subject,omitempty"`
	Text     string         `json:"text,omitempty"`
	HTML     string         `json:"html,omitempty"`
	Template string         `json:"template,omitempty"` // e.g. "login_notification", "verify_email", "forgot_password", "profile_updated"
	Data     map[string]any `json:"data,omitempty"`
}
