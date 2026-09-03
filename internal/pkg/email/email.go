package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"

	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/internal/pkg/utils"
)

func sendEmail(toEmail, subject, htmlContent string) error {
	var brevoErr, resendErr error

	// Primary: Brevo API
	brevoFrom := config.GetEnv("BREVO_FROM_EMAIL", "")
	if brevoFrom == "" {
		brevoFrom = config.GetEnv("RESEND_FROM_EMAIL", "")
	}

	brevoErr = sendViaBrevo(toEmail, brevoFrom, subject, htmlContent)
	if brevoErr == nil {
		return nil
	}

	// Fallback: Resend API
	resendFrom := config.GetEnv("RESEND_FROM_EMAIL", brevoFrom)
	resendErr = sendViaResend(toEmail, resendFrom, subject, htmlContent)
	if resendErr == nil {
		return nil
	}

	return fmt.Errorf("brevo primary failed: %w; resend fallback failed: %v", brevoErr, resendErr)
}

func SendPasswordResetOTP(toEmail, otp string) error {
	if _, err := mail.ParseAddress(toEmail); err != nil {
		return fmt.Errorf("invalid recipient email address: %w", err)
	}

	renderedHTML, err := utils.RenderEmbeddedTemplate("password_reset.html", map[string]any{"OTP": otp, "ExpiryMinutes": 15})
	if err != nil {
		return fmt.Errorf("failed to render password reset template: %w", err)
	}

	return sendEmail(toEmail, "Password reset OTP", renderedHTML)
}

func SendOrganizationInvitation(toEmail, organizationName, role, inviteLink, tempPassword string) error {
	if _, err := mail.ParseAddress(toEmail); err != nil {
		return fmt.Errorf("invalid recipient email address: %w", err)
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

	return sendEmail(toEmail, "Organization invitation", renderedHTML)
}

func SendEmailVerificationOTP(toEmail, otp string, expiryMinutes int) error {
	if _, err := mail.ParseAddress(toEmail); err != nil {
		return fmt.Errorf("invalid recipient email address: %w", err)
	}

	renderedHTML, err := utils.RenderEmbeddedTemplate("email_verification.html", map[string]any{"OTP": otp, "ExpiryMinutes": expiryMinutes})
	if err != nil {
		return fmt.Errorf("failed to render verification email template: %w", err)
	}

	return sendEmail(toEmail, "Verify your email address", renderedHTML)
}

func sendViaBrevo(toEmail, fromEmail, subject, htmlContent string) error {
	apiKey := config.GetEnv("BREVO_API_KEY", "")
	if apiKey == "" {
		return fmt.Errorf("brevo configuration is incomplete (BREVO_API_KEY missing)")
	}
	if fromEmail == "" {
		return fmt.Errorf("brevo sender email address is not configured")
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

func sendViaResend(toEmail, fromEmail, subject, htmlContent string) error {
	apiKey := config.GetEnv("RESEND_API_KEY", "")
	if apiKey == "" {
		return fmt.Errorf("resend configuration is incomplete (RESEND_API_KEY missing)")
	}
	if fromEmail == "" {
		return fmt.Errorf("resend sender email address is not configured")
	}

	payload := map[string]any{
		"from":    fromEmail,
		"to":      []string{toEmail},
		"subject": subject,
		"html":    htmlContent,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return fmt.Errorf("resend api returned status %d", response.StatusCode)
	}

	return nil
}
