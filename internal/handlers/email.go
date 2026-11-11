package handlers

import (
	"fmt"
	"log"
	"net/smtp"
)

func (a *API) sendEmail(subject, htmlBody, to string) error {
	if a.Cfg.EmailHost == "" || a.Cfg.EmailUser == "" {
		log.Printf("email skipped (not configured): to=%s subject=%s", to, subject)
		return nil
	}
	from := a.Cfg.EmailUser
	addr := fmt.Sprintf("%s:%d", a.Cfg.EmailHost, a.Cfg.EmailPort)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		from, to, subject, htmlBody))
	auth := smtp.PlainAuth("", a.Cfg.EmailUser, a.Cfg.EmailPass, a.Cfg.EmailHost)
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}
