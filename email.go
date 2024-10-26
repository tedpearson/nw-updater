package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/gomail.v2"

	"nw-updater/decrypt"
	"nw-updater/institution"
)

type EmailConfig struct {
	Host              string
	Port              int
	EncryptedPassword string `yaml:"encrypted_password"`
	From              string
	To                string
}

//goland:noinspection GoUnhandledErrorResult
func Email(ec EmailConfig, d decrypt.Decryptor, e error) error {
	fmt.Printf("Error, sending email: %s", e)
	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("nw-updater <%s>", ec.From))
	m.SetHeader("To", ec.To)
	m.SetHeader("Subject", "nw-updater error")
	body := &strings.Builder{}
	body.WriteString("<p><b>nw-updater had errors:</b></p><p/>\n")
	for i, theError := range UnwrapError(e) {
		fmt.Fprintf(body, "<p>%s</b>\n", theError.Error())
		var e institution.Error
		if errors.As(theError, &e) {
			stackHtml := strings.ReplaceAll(string(e.Stacktrace), "\n", "<br>\n")
			stackHtml = strings.ReplaceAll(stackHtml, "\t", "&nbsp;&nbsp;&nbsp;&nbsp;")
			fmt.Fprintf(body, "<p>%s</p>\n", stackHtml)
			filename := fmt.Sprintf("%d.png", i)
			fmt.Fprintf(body, "<p><img src=\"cid:%s\" width=\"600\"></p>\n", filename)

			m.Embed(filename, gomail.SetCopyFunc(func(w io.Writer) error {
				_, err := w.Write(e.Screenshot)
				return err
			}))
		}
	}
	m.SetBody("text/html", body.String())
	dialer := gomail.NewDialer(ec.Host, ec.Port, ec.From, d.Decrypt(ec.EncryptedPassword))
	return dialer.DialAndSend(m)
}

func UnwrapError(e error) []error {
	result := make([]error, 0, 1)
	var m *institution.MultiError
	if errors.As(e, &m) {
		for _, e2 := range m.Errors {
			result = append(result, UnwrapError(e2)...)
		}
	} else {
		result = append(result, e)
	}
	return result
}
