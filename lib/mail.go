package lib

import (
	"fmt"
	"net/smtp"
)

func (c *Controller) SendOneTimeLoginCode(toEmail string, code string) error {
	host := c.Config.SMTP_HOST
	port := c.Config.SMTP_PORT
	username := c.Config.SMTP_USERNAME
	password := c.Config.SMTP_PASSWORD

	if host == "" {
		return fmt.Errorf("SMTP_HOST is not configured")
	}
	if port == 0 {
		port = 25
	}

	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	from := username
	if from == "" {
		from = "no-reply@localhost"
	}
	
	msg := []byte(
		"To: " + toEmail + "\r\n" +
		"From: " + from + "\r\n" +
		"Subject: [Simple KVS Registry] Your One-Time Login Code\r\n" +
		"\r\n" +
		"Your one-time login code is: " + code + "\r\n" +
		"\r\n" +
		"This code will expire in 10 minutes.\r\n",
	)

	return smtp.SendMail(addr, auth, from, []string{toEmail}, msg)
}

func (c *Controller) SendRegistrationCode(toEmail string, code string) error {
	host := c.Config.SMTP_HOST
	port := c.Config.SMTP_PORT
	username := c.Config.SMTP_USERNAME
	password := c.Config.SMTP_PASSWORD

	if host == "" {
		return fmt.Errorf("SMTP_HOST is not configured")
	}
	if port == 0 {
		port = 25
	}

	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	from := username
	if from == "" {
		from = "no-reply@localhost"
	}
	
	msg := []byte(
		"To: " + toEmail + "\r\n" +
		"From: " + from + "\r\n" +
		"Subject: [Simple KVS Registry] Complete Your Registration\r\n" +
		"\r\n" +
		"Your registration code is: " + code + "\r\n" +
		"\r\n" +
		"This code will expire in 30 minutes.\r\n",
	)

	return smtp.SendMail(addr, auth, from, []string{toEmail}, msg)
}
