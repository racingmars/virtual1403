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

package scanner

import (
	"encoding/hex"
	"errors"
	"log"
	"net"
	"os"
	"regexp"
	"time"
)

// PrinterHandler interface receives the output of printer output parsing.
type PrinterHandler interface {
	AddLine(line string, linefeed bool)
	PageBreak()
	EndOfJob(jobinfo string)
}

const maxLineLen = 132

const (
	charLF byte = 0xA
	charFF byte = 0xC
	charCR byte = 0xD
)

type stateFunc func(*scanner, byte) stateFunc

type scanner struct {
	conn     net.Conn
	nextfunc stateFunc
	pos      int
	curline  [maxLineLen]byte
	prevline string
	handler  PrinterHandler
	newjob   bool
	trace    bool
}

// Scan will read from a net.Conn, conn, which should be sent data from
// Hercules printer output. It will output lines (trimmed to 132 characters
// if necessary) and page breaks and identify the end of jobs in the printer
// data stream.
func Scan(conn net.Conn, handler PrinterHandler, trace bool) error {
	var s scanner
	s.conn = conn
	s.handler = handler
	s.nextfunc = getNextByte
	s.newjob = true
	s.trace = trace

	nextByte := make([]byte, 1)
	for {
		// If we are in a job, assume the job is done if we don't receive the
		// next character within half a second.
		if !s.newjob {
			if err := s.conn.SetReadDeadline(time.Now().Add(
				500 * time.Millisecond)); err != nil {
				log.Printf("ERROR: couldn't set read deadline: %v", err)
			}
		}
		n, err := s.conn.Read(nextByte)
		if err != nil && errors.Is(err, os.ErrDeadlineExceeded) {
			s.emitLine(true)
			s.endJob(true)
		} else if err != nil {
			return err
		} else if n != 1 {
			log.Printf(
				"ERROR: read 0 bytes when expecting 1; continuing read loop")
		} else {
			s.nextfunc = s.nextfunc(&s, nextByte[0])
		}
	}
}

func (s *scanner) emitLine(linefeed bool) {
	// Trace output for the raw line
	if s.trace {
		log.Printf("TRACE: (lf: %v) scanner got line: %s", linefeed,
			hex.EncodeToString(s.curline[:s.pos]))
	}

	// We need to build a valid UTF-8 string. For now we'll handle a couple
	// mainframe-specific characters we might see, but someday probably need
	// to make a general Hecules-default-to-UTF-8 table.
	utf8runes := make([]rune, 0, len(s.curline))
	for i := 0; i < s.pos; i++ {
		var r rune
		switch s.curline[i] {
		case 0x5e:
			r = '¬'
		case 0xd6:
			r = '¢'
		case 0x9f:
			r = '©'
		default:
			if s.curline[i] > 0x7F {
				log.Printf(
					"DEBUG: got character %02x, need to add mapping\n",
					s.curline[i])
			}
			r = rune(s.curline[i])
		}
		utf8runes = append(utf8runes, r)
	}
	s.prevline = string(utf8runes)
	s.handler.AddLine(s.prevline, linefeed)
	s.pos = 0

}

// This regular expression, *if immediately followed by a LF+FF*, indicates
// end of job from the Moseley MVS 3.8J sysgen and TK4-.
var eojRegexp = regexp.MustCompile(
	`(?m)\*+.+END.+(JOB|STC)\D+(\d+)\s+(\S+)\s+.+ROOM.+END.+\*+`)

// When we emit a line and page together (e.g. we got a LF followed by FF),
// we might be at the end of the job, so we'll check for the end of the
// separator page.
func (s *scanner) emitLineAndPage() {
	s.emitLine(true)
	if s.trace {
		log.Printf("TRACE: scanner checking for end of job on line: %s",
			s.prevline)
	}
	if eojRegexp.MatchString(s.prevline) {
		s.endJob(false)
	} else {
		s.handler.PageBreak()
	}
}

func (s *scanner) endJob(wasTimeout bool) {
	jobinfo := ""

	// If end of job was due to end-of-job line, not a read timeout, we'll try
	// to populate additional job info.
	if !wasTimeout {
		matches := eojRegexp.FindStringSubmatch(s.prevline)
		if len(matches) > 1 {
			// get first letter, e.g. J(ob) or S(tc)
			jobinfo = string(matches[1][0])
		}
		if len(matches) > 2 {
			// Should be the job number
			jobinfo = jobinfo + matches[2]
		}
		if len(matches) > 3 {
			// Should be the job name
			jobinfo = jobinfo + "_" + matches[3]
		}
	}

	s.handler.EndOfJob(jobinfo)
	s.prevline = ""
	s.pos = 0
	s.newjob = true

	// No timeout for the next read awaiting beginning of the next job
	if err := s.conn.SetReadDeadline(time.Time{}); err != nil {
		log.Printf("ERROR: couldn't clear read deadline: %v", err)
	}
}
