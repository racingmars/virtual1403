// Copyright 2022 Matthew R. Wilson <mwilson@mattwilson.org>
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
	"io"
	"log"
)

type fileStateFunc func(*fileScanner, rune) fileStateFunc

type fileScanner struct {
	buf      *bufio.Reader
	nextfunc fileStateFunc
	pos      int
	curline  [maxLineLen]rune
	handler  PrinterHandler
	trace    bool
}

// ScanUTF8Single reads input from a reader (typically local file) and prints
// the entire contents to the handler. No job separation is attempted. The
// input file is assumed to be UTF-8 (compatible with US-ASCII) encoded.
func ScanUTF8Single(r io.Reader, jobname string, handler PrinterHandler,
	trace bool) error {
	b := bufio.NewReader(r)

	var s fileScanner
	s.buf = b
	s.handler = handler
	s.trace = trace
	s.nextfunc = fileGetNextByte

	for {
		nextRune, _, err := s.buf.ReadRune()
		if err == io.EOF {
			if s.pos > 0 {
				s.emitLine()
			}
			handler.EndOfJob(jobname)
			return nil
		}
		if err != nil {
			return err
		}
		s.nextfunc = s.nextfunc(&s, nextRune)
	}
}

func (s *fileScanner) emitLine() {
	// Trace output for the raw line
	if s.trace {
		log.Printf("TRACE: scanner got line: %U", s.curline[:s.pos])
	}
	s.handler.AddLine(string(s.curline[:s.pos]), true)
	s.pos = 0
}

func (s *fileScanner) emitLineAndPage() {
	s.emitLine()
	s.handler.PageBreak()
}
