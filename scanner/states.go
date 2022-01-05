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
		log.Printf(
			"INFO:  [%s] receiving data from Hercules for new print job",
			s.tag)
		s.newjob = false
	}

	switch b {
	case charLF:
		if s.trace {
			log.Printf("TRACE: [%s] scanner got LF in getNextByte", s.tag)
		}
		return haveLF
	case charCR:
		if wasNewJob {
			// if the very first byte we receive is a carriage return, then
			// it's probably from VM resetting the carriage after the previous
			// job. We're already in a new job and the carriage is at position
			// 0, so we'll count this as still being a new job (as we still
			// want to trigger the special beginning-of-job form feed
			// handling, since that's probably what's coming next).
			if s.trace {
				log.Printf("TRACE: [%s] ignoring CR at beginning of job", s.tag)
			}
			s.newjob = true
			return getNextByte
		}
		if s.trace {
			log.Printf("TRACE: [%s] scanner got CR in getNextByte", s.tag)
		}
		return haveCR
	case charFF:
		if s.trace {
			log.Printf("TRACE: [%s] scanner got FF in getNextByte", s.tag)
		}
		if wasNewJob {
			// if the very first byte we receive is a form feed, then it's
			// probably from VM ejecting the previous job (since, for some
			// reason, it doesn't eject jobs right after they finish). We're
			// already starting on a new page, so we'll just suppress it.
			if s.trace {
				log.Printf("TRACE: [%s] ignoring FF at beginning of job", s.tag)
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
			log.Printf("TRACE: [%s] scanner got CR in haveCR", s.tag)
		}
		s.emitLine(false)
		return haveCR
	case charLF:
		if s.trace {
			log.Printf("TRACE: [%s] scanner got LF in haveCR", s.tag)
		}
		s.emitLine(true)
		return getNextByte
	case charFF:
		if s.trace {
			log.Printf("TRACE: [%s] scanner got FF in haveCR", s.tag)
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
			log.Printf("TRACE: [%s] scanner got CR in haveLF", s.tag)
		}
		s.emitLine(true)
		return getNextByte
	case charLF:
		if s.trace {
			log.Printf("TRACE: [%s] scanner got LF in haveLF", s.tag)
		}
		s.emitLine(true)
		return haveLF
	case charFF:
		if s.trace {
			log.Printf("TRACE: [%s] scanner got FF in haveLF", s.tag)
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
