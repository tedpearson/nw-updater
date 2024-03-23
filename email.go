package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/smtp"

	"nw-updater/decrypt"
	"nw-updater/institution"
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
	fmt.Fprint(msg, "Subject: nw-updater error\r\n")
	fmt.Fprint(msg, "MIME-Version: 1.0\r\n")
	writer := multipart.NewWriter(msg)
	boundary := writer.Boundary()
	fmt.Fprintf(msg, "Content-Type: multipart/mixed; boundary=%s\r\n", boundary)
	fmt.Fprintf(msg, "--%s\r\n", boundary)
	fmt.Fprint(msg, "nw-updater had errors:\r\n\r\n")
	for _, theError := range errs {
		fmt.Fprintf(msg, "%s\r\n", theError.Error())
		var e institution.Error
		if errors.As(theError, &e) {
			fmt.Fprintf(msg, "%s\r\n", e.Stacktrace)
		}
	}
	for i, theError := range errs {
		var e institution.Error
		if errors.As(theError, &e) {
			fmt.Fprintf(msg, "\r\n\r\n--%s\r\n", boundary)
			fmt.Fprintf(msg, "Content-Type: %s\r\n", http.DetectContentType(e.Screenshot))
			fmt.Fprintf(msg, "Content-Transfer-Encoding: base64\r\n")
			fmt.Fprintf(msg, "Content-Disposition: attachment; filename=%d.png\r\n\r\n", i)
			encoder := base64.NewEncoder(base64.StdEncoding, msg)
			encoder.Write(e.Screenshot)
			encoder.Close()
			fmt.Fprintf(msg, "\r\n\r\n--%s", boundary)
		}
		fmt.Fprint(msg, "--")
	}
	auth := smtp.PlainAuth("", e.From, d.Decrypt(e.EncryptedPassword), e.Host)
	return smtp.SendMail(e.Host+":"+e.Port, auth, e.From, []string{e.To}, msg.Bytes())
}
