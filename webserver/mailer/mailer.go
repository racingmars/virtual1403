package mailer

// Copyright 2021 Matthew R. Wilson <mwilson@mattwilson.org>
//
// This file is part of virtual1403
// <https://github.com/racingmars/virtual1403>.
//
// virtual1403 is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// virtual1403 is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with virtual1403. If not, see <https://www.gnu.org/licenses/>.

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"mime/quotedprintable"
	"net/smtp"
	"net/textproto"
	"regexp"
	"time"
)

type Config struct {
	Disable     bool   `yaml:"disable"`
	FromAddress string `yaml:"from_address"`
	Server      string `yaml:"server"`
	Port        int    `yaml:"port"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
}

func Send(config Config, to, subject, body, filename string,
	attachment []byte) error {

	// For testing the web service without generating any actual mail
	if config.Disable {
		return nil
	}

	var buf bytes.Buffer

	m := multipart.NewWriter(&buf)

	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "From: %s\r\n", config.FromAddress)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC822Z))
	fmt.Fprintf(&buf, "MIME-version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=%s\r\n",
		m.Boundary())
	fmt.Fprintf(&buf, "\r\n")

	headers := make(textproto.MIMEHeader)
	headers.Set("Content-Type", "text/plain; charset=UTF-8")
	headers.Set("Content-Transfer-Encoding", "quoted-printable")
	headers.Set("Content-Disposition", "inline")
	w, err := m.CreatePart(headers)
	if err != nil {
		return err
	}
	qp := quotedprintable.NewWriter(w)
	qp.Write([]byte(body))
	qp.Close()

	headers = make(textproto.MIMEHeader)
	headers.Set("Content-Type", "application/pdf; filename="+filename)
	headers.Set("Content-Transfer-Encoding", "base64")
	headers.Set("Content-Disposition", "attachment; filename="+filename)
	w, err = m.CreatePart(headers)
	if err != nil {
		return err
	}
	if err = wrappedBase64(attachment, w); err != nil {
		return err
	}

	if err = m.Close(); err != nil {
		return err
	}

	// default nil auth will work for SMTP servers that don't require auth
	var auth smtp.Auth
	if config.Username != "" || config.Password != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password,
			config.Server)
	}
	err = smtp.SendMail(fmt.Sprintf("%s:%d", config.Server, config.Port),
		auth, config.FromAddress, []string{to}, buf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func SendVerificationCode(config Config, to, verifyURL string) error {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "From: %s\r\n", config.FromAddress)
	fmt.Fprintf(&buf, "Subject: virtual1403 email verification\r\n")
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC822Z))
	fmt.Fprintf(&buf, "MIME-version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: text/plain\r\n")
	fmt.Fprintf(&buf, "\r\n")

	io.WriteString(&buf,
		"You (hopefully) have signed up for a Virtual1403 account. To "+
			"activate\r\nyour account, please click the link below:\r\n\r\n")
	io.WriteString(&buf, verifyURL+"\r\n\r\n")
	io.WriteString(&buf, "If you were not the one to sign up with this email "+
		"address, no action is\r\nrequired; the account will remain inactive "+
		"and unverified.\r\n")

	// default nil auth will work for SMTP servers that don't require auth
	var auth smtp.Auth
	if config.Username != "" || config.Password != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password,
			config.Server)
	}
	err := smtp.SendMail(fmt.Sprintf("%s:%d", config.Server, config.Port),
		auth, config.FromAddress, []string{to}, buf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

// from https://www.emailregex.com/
var mailRegexp = regexp.MustCompile(`^(?:[a-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+(?:\.[a-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+)*|"(?:[\x01-\x08\x0b\x0c\x0e-\x1f\x21\x23-\x5b\x5d-\x7f]|\\[\x01-\x09\x0b\x0c\x0e-\x7f])*")@(?:(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?|\[(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?|[a-z0-9-]*[a-z0-9]:(?:[\x01-\x08\x0b\x0c\x0e-\x1f\x21-\x5a\x53-\x7f]|\\[\x01-\x09\x0b\x0c\x0e-\x7f])+)\])$`)

// ValidateAddress will return true if the email address appears to be valid.
func ValidateAddress(email string) bool {
	return mailRegexp.MatchString(email)
}

// base64 will encode the input bytes, in, to base64 wrapped at 76 characters
// for email use. The result is written to out.
func wrappedBase64(in []byte, out io.Writer) error {
	// 57 input bytes encodes to 76 unpadded base64 bytes
	const inputBlock = 57

	outbuf := bufio.NewWriter(out)
	pos := 0
	for pos < len(in) {
		var s string
		if len(in)-pos >= inputBlock {
			s = base64.StdEncoding.EncodeToString(in[pos : pos+inputBlock])
			pos += inputBlock
		} else {
			s = base64.StdEncoding.EncodeToString(in[pos:])
			pos += inputBlock
		}
		if _, err := outbuf.WriteString(s + "\r\n"); err != nil {
			return err
		}
	}

	if err := outbuf.Flush(); err != nil {
		return err
	}
	return nil
}
