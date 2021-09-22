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
	"time"
)

type Config struct {
	FromAddress string
	Server      string
	Port        int
	Username    string
	Password    string
}

func Send(config Config, to, subject, body, filename string, attachment []byte) error {
	var buf bytes.Buffer

	buf.WriteString("To: " + to + "\r\n")
	buf.WriteString("From: " + config.FromAddress + "\r\n")
	buf.WriteString("Subject: " + subject + "\r\n")
	buf.WriteString("Date: ")
	buf.WriteString(time.Now().Format(time.RFC822Z))
	buf.WriteString("\r\n")
	buf.WriteString("Content-Type: multipart/mixed\r\n")
	buf.WriteString("\r\n")

	m := multipart.NewWriter(&buf)

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
	headers.Set("Content-Type", "application/pdf")
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

	auth := smtp.PlainAuth("", config.Username, config.Password, config.Server)
	err = smtp.SendMail(fmt.Sprintf("%s:%d", config.Server, config.Port),
		auth, config.FromAddress, []string{to}, buf.Bytes())
	if err != nil {
		return err
	}

	return nil
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
