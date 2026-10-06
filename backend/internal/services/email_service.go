package services

import (
	"fmt"
	"os"

	"github.com/resend/resend-go/v2"
)

type EmailService struct {
	client    *resend.Client
	fromEmail string
	appURL    string
}

func NewEmailService() *EmailService {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		// Mode dégradé : logs seulement
		fmt.Println("⚠️  RESEND_API_KEY not set - emails will be logged only")
		return &EmailService{
			client:    nil,
			fromEmail: "noreply@omlet.app",
			appURL:    os.Getenv("APP_URL"),
		}
	}

	return &EmailService{
		client:    resend.NewClient(apiKey),
		fromEmail: getEnvOrDefault("FROM_EMAIL", "noreply@omlet.app"),
		appURL:    getEnvOrDefault("APP_URL", "http://localhost:3000"),
	}
}

func (s *EmailService) SendVerificationEmail(to, firstName, token string) error {
	verificationURL := fmt.Sprintf("%s/verify-email?token=%s", s.appURL, token)

	subject := "Confirmez votre compte Omlet"
	htmlContent := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<style>
				body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
				.container { max-width: 600px; margin: 0 auto; padding: 20px; }
				.header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
				.content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
				.button { display: inline-block; background: #667eea; color: white; padding: 15px 30px; text-decoration: none; border-radius: 5px; margin: 20px 0; }
				.footer { text-align: center; margin-top: 20px; color: #888; font-size: 12px; }
			</style>
		</head>
		<body>
			<div class="container">
				<div class="header">
					<h1>🐔 Bienvenue sur Omlet !</h1>
				</div>
				<div class="content">
					<p>Bonjour %s,</p>
					<p>Merci de vous être inscrit sur Omlet, votre assistant intelligent pour gérer votre poulailler connecté.</p>
					<p>Pour activer votre compte, veuillez confirmer votre adresse email en cliquant sur le bouton ci-dessous :</p>
					<p style="text-align: center;">
						<a href="%s" class="button">Confirmer mon email</a>
					</p>
					<p>Ou copiez ce lien dans votre navigateur :</p>
					<p style="word-break: break-all; color: #667eea;">%s</p>
					<p>Ce lien est valide pendant 24 heures.</p>
					<p>Si vous n'avez pas créé de compte, vous pouvez ignorer cet email.</p>
					<p>À bientôt,<br>L'équipe Omlet</p>
				</div>
				<div class="footer">
					<p>© 2025 Omlet - Gestion intelligente de poulailler</p>
				</div>
			</div>
		</body>
		</html>
	`, firstName, verificationURL, verificationURL)

	// Mode dégradé : log seulement
	if s.client == nil {
		fmt.Printf("📧 [EMAIL MOCK] Would send to %s\n", to)
		fmt.Printf("   Subject: %s\n", subject)
		fmt.Printf("   Verification URL: %s\n", verificationURL)
		return nil
	}

	// Envoi réel via Resend
	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{to},
		Subject: subject,
		Html:    htmlContent,
	}

	_, err := s.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	fmt.Printf("✅ Verification email sent to %s\n", to)
	return nil
}

func (s *EmailService) SendPasswordResetEmail(to, firstName, token string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.appURL, token)

	subject := "Réinitialisation de votre mot de passe Omlet"
	htmlContent := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<style>
				body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
				.container { max-width: 600px; margin: 0 auto; padding: 20px; }
				.header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
				.content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
				.button { display: inline-block; background: #667eea; color: white; padding: 15px 30px; text-decoration: none; border-radius: 5px; margin: 20px 0; }
				.footer { text-align: center; margin-top: 20px; color: #888; font-size: 12px; }
				.warning { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; }
			</style>
		</head>
		<body>
			<div class="container">
				<div class="header">
					<h1>🔐 Réinitialisation du mot de passe</h1>
				</div>
				<div class="content">
					<p>Bonjour %s,</p>
					<p>Vous avez demandé à réinitialiser votre mot de passe Omlet.</p>
					<p>Cliquez sur le bouton ci-dessous pour créer un nouveau mot de passe :</p>
					<p style="text-align: center;">
						<a href="%s" class="button">Réinitialiser mon mot de passe</a>
					</p>
					<p>Ou copiez ce lien dans votre navigateur :</p>
					<p style="word-break: break-all; color: #667eea;">%s</p>
					<div class="warning">
						<strong>⚠️ Important :</strong> Ce lien expire dans 1 heure.
					</div>
					<p>Si vous n'avez pas demandé cette réinitialisation, ignorez cet email et votre mot de passe restera inchangé.</p>
					<p>Cordialement,<br>L'équipe Omlet</p>
				</div>
				<div class="footer">
					<p>© 2025 Omlet - Gestion intelligente de poulailler</p>
				</div>
			</div>
		</body>
		</html>
	`, firstName, resetURL, resetURL)

	// Mode dégradé : log seulement
	if s.client == nil {
		fmt.Printf("📧 [EMAIL MOCK] Would send password reset to %s\n", to)
		fmt.Printf("   Reset URL: %s\n", resetURL)
		return nil
	}

	// Envoi réel via Resend
	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{to},
		Subject: subject,
		Html:    htmlContent,
	}

	_, err := s.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	fmt.Printf("✅ Password reset email sent to %s\n", to)
	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
