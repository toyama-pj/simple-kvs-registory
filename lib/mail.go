package lib

import (
	"errors"
	"fmt"
	"net/smtp"
	"strings"
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
		auth = &unconstrainedAuth{
			username: username,
			password: password,
			host:     host,
		}
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
		auth = &unconstrainedAuth{
			username: username,
			password: password,
			host:     host,
		}
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

// unconstrainedAuth is a custom SMTP authenticator that supports both PLAIN and LOGIN,
// and doesn't restrict to TLS-only like the standard PlainAuth does.
type unconstrainedAuth struct {
	username string
	password string
	host     string
	authType string
}

func (a *unconstrainedAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	// サーバーがサポートしている認証方式を確認
	var hasPlain, hasLogin bool
	for _, auth := range server.Auth {
		if auth == "PLAIN" {
			hasPlain = true
		} else if auth == "LOGIN" {
			hasLogin = true
		}
	}

	if hasPlain {
		a.authType = "PLAIN"
		resp := []byte("\x00" + a.username + "\x00" + a.password)
		return "PLAIN", resp, nil
	} else if hasLogin {
		a.authType = "LOGIN"
		return "LOGIN", []byte{}, nil
	}

	return "", nil, errors.New("smtp: server doesn't support PLAIN or LOGIN auth")
}

func (a *unconstrainedAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	if a.authType == "LOGIN" {
		prompt := strings.ToLower(string(fromServer))
		if strings.Contains(prompt, "username") {
			return []byte(a.username), nil
		} else if strings.Contains(prompt, "password") {
			return []byte(a.password), nil
		}
	}
	return nil, nil
}
