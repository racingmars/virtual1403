package main

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
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"

	"github.com/racingmars/virtual1403/vprinter"
	"github.com/racingmars/virtual1403/webserver/mailer"
)

// printjob is the handler for the primary use case of the server: receive
// the text of a print job and generate a PDF. Clients send the data in the
// request body as a series of print directives. Print directives must be
// valid UTF-8 strings separated by CRLF, CR, or LF. Each print directive
// contains a one-letter prefix (L, O, P), followed by a colon (:), followed
// by the (optional) data for the directive. Each HTTP POST represents one
// print job.
//
// Request requirements:
//
// 1. The HTTP method must be POST.
// 2. The request must be authenticated with a user's API key as a bearer
//    token. That is, the request must contain the header:
//    Authorization: Bearer <api key>
// 3. The Content-Type header value must be "text/x-print-job".
// 4. The request body must be compressed using the zstd compression
//    algorithm.
// 5. The Content-Encoding header value must be "zstd".
//
// Print directives:
//
// The (decompressed) request body may contain the following print directives:
//
// L:[line data]  - One line of text to print, after which the next line will
//                  print on the next line on the page. <line data> must be a
//                  valid UTF-8 string, and will be trimmed to 132 characters.
//                  <line data> may be empty, in which case a blank line will
//                  be printed.
// O:[line data]  - One line of text to print, after which the "virtual
//                  carriage" will return to the beginning of the line but NOT
//                  advance to the next line. That is, this is "overstrikable"
//                  text, so the *next* O: or L: directive will print the text
//                  over top of *this* text. Otherwise, this behaves like L:
//                  directives.
// P:               Page break. This will advance the virtual printer to the
//                  next page. Any data on a P: directive is ignored.
// J:[job data]   - Job data. This optional component may contain a string up
//                  to 25 characters long, containing the characters
//                  [a-zA-Z0-9_] with an identifier for the job that may be
//                  included in the generated filename. If there are multiple
//                  J: directives, only the last one is used.
//
// Responses:
//
// 200 - OK
//       The request was processed successfully and the PDF of the print job
//       has been sent to the user.
// 400 - Bad Request
//       The server was unable to process the request body due to invalid
//       print directives (unknown directive or invalid UTF-8 string) or error
//       during zstd decompression.
// 401 - Unauthorized
//       Either the Authorization header is missing from the request, or the
//       supplied API key is invalid.
// 405 - Method Not Allowed
//       Returned when the HTTP request method is not POST.
// 415 - Unsupported Media Type
//       Returned when Content-Type is not text/x-print-job or
//       Content-Encoding is not zstd.
// 429 - Too Many Requests
//       The user has exceeded their quota of print jobs in a period of time.
// 500 - Internal Server Error
//       The virtual 1403 printer experienced a paper jam and is awaiting
//       operator intervention.
func (a *application) printjob(w http.ResponseWriter, r *http.Request) {
	// We only accept POST requests.
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.Header().Set("Accept-Encoding", "zstd")
		w.Header().Set("Accept", "text/x-print-job")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate
	authHdr := r.Header.Get("Authorization")
	authHdr = strings.TrimPrefix(authHdr, "Bearer ")
	user, err := a.db.GetUserForAccessKey(authHdr)
	if err != nil {
		log.Printf("INFO:  unauthorized web service call from %s",
			r.RemoteAddr)
		http.Error(w, "Authentication failure", http.StatusUnauthorized)
		return
	}
	if !user.Enabled {
		http.Error(w, "User's account is disabled", http.StatusForbidden)
		return
	}
	if !user.Verified {
		http.Error(w, "User's email address has not been verified",
			http.StatusForbidden)
		return
	}

	// Now that we have confirmed this is a valid user, if we are limiting
	// concurrency of the print service, we will wait until a seat is
	// available.
	if a.printerSeats != nil {
		// Writing to the channel will block if the queue is full.
		a.printerSeats <- true

		// when this function ends, we release the user's seat
		defer func() {
			<-a.printerSeats
		}()
	}

	// Enforce quotas
	if _, _, err := a.checkQuota(user.Email); err == errQuotaExceeded {
		log.Printf("INFO:  user %s attempted to print over quota", user.Email)
		http.Error(w, a.quotaString(), http.StatusTooManyRequests)
		return
	} else if err != nil {
		log.Printf("ERROR: db error calculating user quota: %v", err)
		http.Error(w, "internal db error", http.StatusInternalServerError)
		return
	}

	// Content must be zstd-compressed.
	if r.Header.Get("Content-Encoding") != "zstd" {
		w.Header().Set("Accept-Encoding", "zstd")
		w.Header().Set("Accept", "text/x-print-job")
		http.Error(w, "Requests must use zstd compression",
			http.StatusUnsupportedMediaType)
		return
	}

	// Content must be of type text/x-print-job.
	if r.Header.Get("Content-Type") != "text/x-print-job" {
		w.Header().Set("Accept-Encoding", "zstd")
		w.Header().Set("Accept", "text/x-print-job")
		http.Error(w, "Requests must be of type text/x-print-job",
			http.StatusUnsupportedMediaType)
		return
	}

	// Set up decompressor on the request body.
	d, err := zstd.NewReader(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to begin zstd decoding: %v", err),
			http.StatusBadRequest)
		return
	}
	defer d.Close()

	// Create our virtual printer.
	job, err := vprinter.New1403(a.font)
	if err != nil {
		log.Printf("ERROR: couldn't create virtual printer: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Process the directives in the request body and send them to the
	// virtual printer.
	pageQuota := a.quotaPages
	maxLines := a.maxLinesPerJob
	// Unlimited users are trusted and we apply no limits
	if user.Unlimited {
		pageQuota = 0
		maxLines = 0
	}
	jobinfo, err := processPrintDirectives(d, job, pageQuota,
		maxLines)
	if err != nil {
		log.Printf("INFO:  invalid print directives from %s: %v",
			user.Email, err)
		http.Error(w, fmt.Sprintf("Invalid data: %v", err),
			http.StatusBadRequest)
		return
	}

	// Create the PDF
	var pdfBuffer bytes.Buffer
	var pagecount int
	if pagecount, err = job.EndJob(&pdfBuffer); err != nil {
		log.Printf("ERROR: couldn't create PDF: %v", err)
		http.Error(w, fmt.Sprintf("error creating PDF: %v", err),
			http.StatusInternalServerError)
		return
	}

	jobtag := jobinfo
	if jobtag != "" {
		jobtag = jobtag + "-"
	}
	jobname := fmt.Sprintf("%s%s", jobtag,
		time.Now().UTC().Format("2006-01-02T15:04:05Z"))

	attachmentName := fmt.Sprintf("virtual1403_%s.pdf", jobname)

	err = mailer.Send(a.mailconfig, user.Email,
		"Virtual 1403 printout "+jobinfo,
		"The intern in the machine room has carefully collated your job and "+
			"prepared it for delivery. Please find it attached to this "+
			"message.\r\n\r\n"+
			"The font used in the attached PDF is 1403 Vintage Mono from "+
			"Slanted Hall, used under license.\r\n",
		attachmentName, pdfBuffer.Bytes())
	if err != nil {
		log.Printf("ERROR: error sending email: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("INFO:  sent %d pages to %s", pagecount, user.Email)

	// Try to log the job to the database
	if err = a.db.LogJob(user.Email, jobinfo, pagecount); err != nil {
		log.Printf("ERROR: couldn't log job: %v", err)
	}

	// HTTP 200 will be returned if we make it this far.
}

// jobInfoRegex matches valid/allowed job info data
var jobInfoRegex = regexp.MustCompile(`^[a-zA-z0-9_]{0,25}$`)

// processPrintDirectives will apply print directives to the virtual printer
// job, returning an error if the input data is invalid. Processing will stop
// after maxpages if maxpages > 0 or after maxlines if maxlines > 0.
func processPrintDirectives(r io.Reader, job vprinter.Job,
	maxpages, maxlines int) (string, error) {

	var jobinfo string
	var lines int
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if len(line) < 2 {
			return "", errors.New("line received without directive")
		}
		directive := line[0:2]
		param := line[2:]

		// In all cases, param must be a valid UTF-8 string <= 132 runes, so
		// we'll take care of that now.
		if !utf8.ValidString(param) {
			return "", errors.New("invalid UTF-8 string")
		}

		// Trim to 132 runes
		param = trimToRuneLen(param, 132)

		var pages int
		switch directive {
		case "L:":
			pages = job.AddLine(param, true)
		case "O:":
			pages = job.AddLine(param, false)
		case "P:":
			pages = job.NewPage()
		case "J:":
			if !jobInfoRegex.MatchString(param) {
				return "", errors.New("invalid job data directive")
			}
			jobinfo = param
		default:
			return "", errors.New("invalid directive received")
		}
		lines++

		if maxpages > 0 && pages > maxpages {
			break
		}
		if maxlines > 0 && lines > maxlines {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return jobinfo, nil
}

// trimToRuneLen trims the input string, str, to no more than n runes. The
// input string must be a valid UTF-8 string; the behavior of this function
// is undefined if not.
func trimToRuneLen(str string, n int) string {
	if utf8.RuneCountInString(str) <= n {
		return str
	}

	runes := 0
	i := 0
	for i < len(str) && runes < n {
		_, size := utf8.DecodeRuneInString(str[i:])
		runes++
		i += size
	}
	return str[0:i]
}
