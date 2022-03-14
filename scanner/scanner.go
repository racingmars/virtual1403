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
	tag      string
}

// Scan will read from a net.Conn, conn, which should be sent data from
// Hercules printer output. It will output lines (trimmed to 132 characters
// if necessary) and page breaks and identify the end of jobs in the printer
// data stream.
//
// This function exists for backwards-compatibility and just calls
// ScanWithLogTag with the tag "default"
func Scan(conn net.Conn, handler PrinterHandler, trace bool) error {
	return ScanWithLogTag(conn, handler, trace, "default")
}

// Scan will read from a net.Conn, conn, which should be sent data from
// Hercules printer output. It will output lines (trimmed to 132 characters
// if necessary) and page breaks and identify the end of jobs in the printer
// data stream.
func ScanWithLogTag(conn net.Conn, handler PrinterHandler, trace bool,
	tag string) error {

	var s scanner
	s.conn = conn
	s.handler = handler
	s.nextfunc = getNextByte
	s.newjob = true
	s.trace = trace
	s.tag = tag

	nextByte := make([]byte, 1)
	for {
		// If we are in a job, assume the job is done if we don't receive the
		// next character within half a second.
		if !s.newjob {
			if err := s.conn.SetReadDeadline(time.Now().Add(
				500 * time.Millisecond)); err != nil {
				log.Printf("ERROR: [%s] couldn't set read deadline: %v", tag,
					err)
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
				"ERROR: [%s] read 0 bytes when expecting 1; continuing read loop",
				tag)
		} else {
			if nextByte[0] == 0xFF {
				// This seems to be a control character that VM emits the
				// first time it prints to a printer after IPL. Hercules'
				// ASCII<->EBCDIC mapping has 0xFF on both sides. In
				// ISO-8859-1, 0xFF is ÿ, but that's clearly not a correct
				// mapping from any EBCDIC codepage. Thus, we'll just drop it
				// as a control character that we don't need since it's
				// clearly not meant to be a character from any typical
				// mainframe print job.
				if s.trace {
					log.Printf("TRACE: [%s] ignoring 0xFF control character",
						tag)
				}
				continue
			}
			s.nextfunc = s.nextfunc(&s, nextByte[0])
		}
	}
}

func (s *scanner) emitLine(linefeed bool) {
	// Trace output for the raw line
	if s.trace {
		log.Printf("TRACE: [%s] (lf: %v) scanner got line: %s", s.tag,
			linefeed, hex.EncodeToString(s.curline[:s.pos]))
	}

	// We need to build a valid UTF-8 string. For now we'll handle a couple
	// mainframe-specific characters we might see, but someday probably need
	// to make a general Hercules-default-to-UTF-8 table. See:
	// https://github.com/SDL-Hercules-390/hyperion/blob/master/codepage.c#L99
	utf8runes := make([]rune, 0, len(s.curline))
	for i := 0; i < s.pos; i++ {
		var r rune
		switch s.curline[i] {
		case 0x5e:
			r = '¬'
		case 0xd6:
			r = '¢'
		case 0xd7:
			r = '|'
		case 0x9b:
			r = '^'
		case 0x9f:
			r = '©'
		default:
			if s.curline[i] > 0x7F {
				log.Printf(
					"WARN:  [%s] got character %02x, need to add mapping\n",
					s.tag, s.curline[i])
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
	`(?m)\*+.+END.+(JOB|STC|TSU)\D+(\d+)\s+(\S+)\s+.+ROOM.+END.+\*+`)

// When we emit a line and page together (e.g. we got a LF followed by FF),
// we might be at the end of the job, so we'll check for the end of the
// separator page.
func (s *scanner) emitLineAndPage() {
	s.emitLine(true)
	if s.trace {
		log.Printf("TRACE: [%s] scanner checking for end of job on line: %s",
			s.tag, s.prevline)
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
		log.Printf("ERROR: [%s] couldn't clear read deadline: %v", s.tag, err)
	}
}
