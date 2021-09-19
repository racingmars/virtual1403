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
	"bufio"
	"log"
	"regexp"
)

// PrinterHandler interface receives the output of printer output parsing.
type PrinterHandler interface {
	AddLine(line string)
	PageBreak()
	EndOfJob()
}

const maxLineLen = 132

const (
	charLF byte = 0xA
	charFF byte = 0xC
	charCR byte = 0xD
)

type stateFunc func(*scanner, byte) stateFunc

type scanner struct {
	rdr      *bufio.Reader
	nextfunc stateFunc
	pos      int
	curline  [maxLineLen]byte
	prevline string
	handler  PrinterHandler
}

// Scan will read from a bufio.Reader, r, which should be sent data from
// Hercules printer output. It will output lines (trimmed to 132 characters
// if necessary) and page breaks and identify the end of jobs in the printer
// data stream.
func Scan(r *bufio.Reader, handler PrinterHandler) error {
	var s scanner
	s.rdr = r
	s.handler = handler
	s.nextfunc = getNextByte

	for {
		b, err := s.rdr.ReadByte()
		if err != nil {
			return err
		}
		s.nextfunc = s.nextfunc(&s, b)
	}
}

func (s *scanner) emitLine() {
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
	s.handler.AddLine(s.prevline)
	s.pos = 0
}

// This regular expression, *if immediately followed by a LF+FF*, indicates
// end of job from the Moseley MVS 3.8J sysgen.
var eojRegexp = regexp.MustCompile(`(?m)\*\*\*\*.+END.+(?:JOB|STC).+ROOM.+(?:JOB|STC).+END.+\*\*\*\*`)

// When we emit a line and page together (e.g. we got a LF followed by FF),
// we might be at the end of the job, so we'll check for the end of the
// separator page.
func (s *scanner) emitLineAndPage() {
	s.emitLine()
	if eojRegexp.MatchString(s.prevline) {
		s.endJob()
	} else {
		s.handler.PageBreak()
	}
}

func (s *scanner) endJob() {
	s.handler.EndOfJob()
	s.prevline = ""
	s.pos = 0
}
