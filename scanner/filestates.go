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

import "log"

func fileGetNextByte(s *fileScanner, b rune) fileStateFunc {
	switch b {
	case rune(charLF):
		if s.trace {
			log.Printf("TRACE: scanner got LF in fileGetNextByte")
		}
		s.emitLine()
		return fileGetNextByte
	case rune(charCR):
		if s.trace {
			log.Printf("TRACE: scanner got CR in fileGetNextByte")
		}
		return fileHaveCR
	case rune(charFF):
		if s.trace {
			log.Printf("TRACE: scanner got FF in fileGetNextByte")
		}
		s.emitLineAndPage()
		return fileGetNextByte
	default:
		s.curline[s.pos] = b
		s.pos++
		if s.pos >= maxLineLen {
			return fileDisposeBytes
		}
		return fileGetNextByte
	}
}

// fileDisposeBytes is a state where we discard additional bytes that come in
// and we wait for a control character.
func fileDisposeBytes(s *fileScanner, b rune) fileStateFunc {
	switch b {
	case rune(charCR):
		return fileHaveCR
	case rune(charLF):
		s.emitLine()
		return fileGetNextByte
	case rune(charFF):
		s.emitLineAndPage()
		return fileGetNextByte
	default:
		return fileDisposeBytes
	}
}

// fileHaveCR is a state where we have received a CR control character, and we
// are waiting to see if it is a bare CR, a CRLF, or a sequence of multiple
// CRs. Bare CRs indicate we should overtype the next line on the current
// line.
func fileHaveCR(s *fileScanner, b rune) fileStateFunc {
	switch b {
	case rune(charCR):
		if s.trace {
			log.Printf("TRACE: scanner got CR in fileHaveCR")
		}
		s.emitLine()
		return fileHaveCR
	case rune(charLF):
		if s.trace {
			log.Printf("TRACE: scanner got LF in fileHaveCR")
		}
		s.emitLine()
		return fileGetNextByte
	case rune(charFF):
		if s.trace {
			log.Printf("TRACE: scanner got FF in fileHaveCR")
		}
		s.emitLineAndPage()
		return fileGetNextByte
	default:
		s.emitLine()
		s.curline[s.pos] = b
		s.pos++
		return fileGetNextByte
	}
}
