package main

import (
	"bytes"
	"fmt"
	"net/smtp"

	"nw-updater/decrypt"
)

type EmailConfig struct {
	Host              string
	Port              string
	EncryptedPassword string `yaml:"encrypted_password"`
	From              string
	To                string
}

//goland:noinspection GoUnhandledErrorResult
func Email(e EmailConfig, d decrypt.Decryptor, errs []error) error {
	msg := &bytes.Buffer{}
	fmt.Fprintf(msg, "From: nw-updater <%s>\r\n", e.From)
	fmt.Fprintf(msg, "To: %s\r\n", e.To)
	fmt.Fprint(msg, "Subject: nw-updater error\r\n\r\n")
	fmt.Fprint(msg, "nw-updater had errors:\r\n\r\n")
	for _, theError := range errs {
		_, err := fmt.Fprint(msg, theError.Error())
		if err != nil {
			return err
		}
	}
	auth := smtp.PlainAuth("", e.From, d.Decrypt(e.EncryptedPassword), e.Host)
	return smtp.SendMail(e.Host+":"+e.Port, auth, e.From, []string{e.To}, msg.Bytes())
}
