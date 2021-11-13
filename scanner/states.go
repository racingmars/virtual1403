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

import "log"

// getNextByte represents the "normal" state where we are collecting input
// characters into the current line until we get a control character or
// overflow the current line.
func getNextByte(s *scanner, b byte) stateFunc {
	wasNewJob := s.newjob
	if s.newjob {
		log.Printf("INFO:  receiving data from Hercules for new print job")
		s.newjob = false
	}

	switch b {
	case charLF:
		if s.trace {
			log.Println("TRACE: scanner got LF in getNextByte")
		}
		return haveLF
	case charCR:
		if s.trace {
			log.Println("TRACE: scanner got CR in getNextByte")
		}
		return haveCR
	case charFF:
		if s.trace {
			log.Println("TRACE: scanner got FF in getNextByte")
		}
		if wasNewJob {
			// if the very first byte we receive is a form feed, then it's
			// probably from VM ejecting the previous job (since, for some
			// reason, it doesn't eject jobs right after they finish). We're
			// already starting on a new page, so we'll just suppress it.
			if s.trace {
				log.Println("TRACE: ignoring FF at beginning of job")
			}
			return getNextByte
		}
		s.emitLineAndPage()
		return getNextByte
	default:
		// Add byte to the current line
		s.curline[s.pos] = b
		s.pos++
		// Line can be at most 132 characters
		if s.pos >= maxLineLen {
			return disposeBytes
		}

		return getNextByte
	}
}

// disposeBytes is a state where we discard additional bytes that come in and
// we wait for a control character.
func disposeBytes(s *scanner, b byte) stateFunc {
	switch b {
	case charCR:
		return haveCR
	case charLF:
		return haveLF
	case charFF:
		s.emitLineAndPage()
		return getNextByte
	default:
		return disposeBytes
	}
}

// haveCR is a state where we have received a CR control character, and we are
// waiting to see if it is a bare CR, a CRLF, or a sequence of multiple CRs.
// Bare CRs indicate we should overtype the next line on the current line.
func haveCR(s *scanner, b byte) stateFunc {
	switch b {
	case charCR:
		if s.trace {
			log.Println("TRACE: scanner got CR in haveCR")
		}
		s.emitLine(false)
		return haveCR
	case charLF:
		if s.trace {
			log.Println("TRACE: scanner got LF in haveCR")
		}
		s.emitLine(true)
		return getNextByte
	case charFF:
		if s.trace {
			log.Println("TRACE: scanner got FF in haveCR")
		}
		s.emitLineAndPage()
		return getNextByte
	default:
		s.emitLine(false)
		s.curline[s.pos] = b
		s.pos++
		return getNextByte
	}
}

// haveLF is a state where we have received a LF control character, and we are
// waiting to see if it is a bare LR, or a LFCF, or a sequence of multiple LFs.
func haveLF(s *scanner, b byte) stateFunc {
	switch b {
	case charCR:
		if s.trace {
			log.Println("TRACE: scanner got CR in haveLF")
		}
		s.emitLine(true)
		return getNextByte
	case charLF:
		if s.trace {
			log.Println("TRACE: scanner got LF in haveLF")
		}
		s.emitLine(true)
		return haveLF
	case charFF:
		if s.trace {
			log.Println("TRACE: scanner got FF in haveLF")
		}
		s.emitLineAndPage()
		return getNextByte
	default:
		s.emitLine(true)
		s.curline[s.pos] = b
		s.pos++
		return getNextByte
	}
}
