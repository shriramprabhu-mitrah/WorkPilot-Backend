package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"time"

	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/internal/pkg/utils"
	simplemail "github.com/xhit/go-simple-mail/v2"
)

func SendPasswordResetOTP(toEmail, otp string) error {
	renderedHTML, err := utils.RenderEmbeddedTemplate("password_reset.html", map[string]any{"OTP": otp, "ExpiryMinutes": 15})
	if err != nil {
		return fmt.Errorf("failed to render password reset template: %w", err)
	}

	fromEmail := config.GetEnv("GMAIL_FROM_EMAIL", config.GetEnv("BREVO_FROM_EMAIL", ""))
	if fromEmail == "" {
		return fmt.Errorf("email sender address is not configured")
	}

	subject := "Password reset OTP"
	gmailErr := sendViaGmailSMTP(toEmail, fromEmail, subject, renderedHTML)
	if gmailErr != nil {
		brevoErr := sendViaBrevo(toEmail, fromEmail, subject, renderedHTML)
		if brevoErr != nil {
			return fmt.Errorf("gmail smtp failed: %w; brevo fallback failed: %v", gmailErr, brevoErr)
		}
	}

	return nil
}

func SendOrganizationInvitation(toEmail, organizationName, role, inviteLink, tempPassword string) error {
	if _, err := mail.ParseAddress(toEmail); err != nil {
		return fmt.Errorf("invalid recipient email address: %w", err)
	}

	fromEmail := config.GetEnv("GMAIL_FROM_EMAIL", config.GetEnv("BREVO_FROM_EMAIL", ""))
	if fromEmail == "" {
		return fmt.Errorf("email sender address is not configured")
	}

	renderedHTML, err := utils.RenderEmbeddedTemplate("organization_invitation.html", map[string]any{
		"OrganizationName": organizationName,
		"Role":             role,
		"InviteLink":       inviteLink,
		"TempPassword":     tempPassword,
	})
	if err != nil {
		return fmt.Errorf("failed to render organization invitation template: %w", err)
	}

	subject := "Organization invitation"
	gmailErr := sendViaGmailSMTP(toEmail, fromEmail, subject, renderedHTML)
	if gmailErr != nil {
		brevoErr := sendViaBrevo(toEmail, fromEmail, subject, renderedHTML)
		if brevoErr != nil {
			return fmt.Errorf("gmail smtp failed: %w; brevo fallback failed: %v", gmailErr, brevoErr)
		}
	}

	return nil
}

func SendEmailVerificationOTP(toEmail, otp string, expiryMinutes int) error {
	if _, err := mail.ParseAddress(toEmail); err != nil {
		return fmt.Errorf("invalid recipient email address: %w", err)
	}

	fromEmail := config.GetEnv("GMAIL_FROM_EMAIL", config.GetEnv("BREVO_FROM_EMAIL", ""))
	if fromEmail == "" {
		return fmt.Errorf("email sender address is not configured")
	}

	renderedHTML, err := utils.RenderEmbeddedTemplate("email_verification.html", map[string]any{"OTP": otp, "ExpiryMinutes": expiryMinutes})
	if err != nil {
		return fmt.Errorf("failed to render verification email template: %w", err)
	}

	subject := "Verify your email address"
	gmailErr := sendViaGmailSMTP(toEmail, fromEmail, subject, renderedHTML)
	if gmailErr != nil {
		brevoErr := sendViaBrevo(toEmail, fromEmail, subject, renderedHTML)
		if brevoErr != nil {
			return fmt.Errorf("gmail smtp failed: %w; brevo fallback failed: %v", gmailErr, brevoErr)
		}
	}

	return nil
}

func sendViaGmailSMTP(toEmail, fromEmail, subject, htmlContent string) error {
	host := config.GetEnv("GMAIL_SMTP_HOST", "smtp.gmail.com")
	portString := config.GetEnv("GMAIL_SMTP_PORT", "587")
	username := config.GetEnv("GMAIL_SMTP_USERNAME", "")
	password := config.GetEnv("GMAIL_SMTP_PASSWORD", "")

	if username == "" || password == "" {
		return fmt.Errorf("gmail smtp configuration is incomplete")
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return fmt.Errorf("invalid gmail smtp port: %w", err)
	}

	smtpClient := simplemail.NewSMTPClient()
	smtpClient.Host = host
	smtpClient.Port = port
	smtpClient.Username = username
	smtpClient.Password = password
	smtpClient.Authentication = simplemail.AuthPlain
	smtpClient.KeepAlive = false
	smtpClient.ConnectTimeout = 10 * time.Second
	smtpClient.SendTimeout = 10 * time.Second
	smtpClient.Encryption = simplemail.EncryptionSTARTTLS

	smtpServer, err := smtpClient.Connect()
	if err != nil {
		return err
	}
	defer smtpServer.Close()

	email := simplemail.NewMSG()
	email.SetFrom(fromEmail)
	email.AddTo(toEmail)
	email.SetSubject(subject)
	email.SetBody(simplemail.TextHTML, htmlContent)

	if email.Error != nil {
		return email.Error
	}

	return email.Send(smtpServer)
}

func sendViaBrevo(toEmail, fromEmail, subject, htmlContent string) error {
	apiKey := config.GetEnv("BREVO_API_KEY", "")
	if apiKey == "" {
		return fmt.Errorf("brevo configuration is incomplete")
	}

	payload := map[string]any{
		"sender": map[string]string{
			"email": fromEmail,
		},
		"to":          []map[string]string{{"email": toEmail}},
		"subject":     subject,
		"htmlContent": htmlContent,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, "https://api.brevo.com/v3/smtp/email", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("api-key", apiKey)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return fmt.Errorf("brevo api returned status %d", response.StatusCode)
	}

	return nil
}
