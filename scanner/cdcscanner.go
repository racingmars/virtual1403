// Copyright 2024 William Schaub <william.schaub@gmail.com>
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
	"unicode/utf8"
)

// ScanCDCUTF8Single reads input from a reader (typically local file) and
// prints the entire contents to the handler. No job separation is attempted.
// The input file is assumed to be UTF-8 (compatible with US-ASCII) encoded,
// with the first character of each line being an CDC carriage control effector
// instructions (' ', '1', '0', '-', and '+' are supported).
// and have the same meanings as ASA
// CDC instructions '8', '2', '3', '4', '5', '6' are also supported and are described in
// under Appendix H of https://bitsavers.org/pdf/cdc/Tom_Hunter_Scans/60459680-NOS2_Vol3_Sys_Cmds_RevR.pdf 
// on PDF page 765
func ScanCDCUTF8Single(r io.Reader, jobname string, handler PrinterHandler,
	trace bool) error {

	linenum := 0
    formline := 0 //line number of current page
	var prevline string
	scanner := bufio.NewScanner(r)

    //Closure function to do a vertical tab operation
    jumptoline := func(lin int) {
        if (lin <= formline) {
        return;
        }
        if (lin > 66 ) {
            return;
        }
        for formline < lin {
            handler.AddLine("", true)
            formline++
        }
        return
    }

	for scanner.Scan() {
		linenum++
        formline++
        if(formline > 66) {
            formline = 0;
            //log.Printf("new page at linenum = %d", linenum)
        }
        //log.Printf("form line %d$",formline)
        //log.Printf("linenum = %d", linenum)

		line := scanner.Text()
		if len(line) == 0 {
			// This is blank line that doesn't even include a carriage
			// control character. Technically this is incorrect, but we'll
			// be lenient and just treat it as a blank line with the regular
			// " " carriage control character
			line = " "
		}
		control, size := utf8.DecodeRuneInString(line)
		if control == utf8.RuneError {
			// If the user is providing an input file with an invalid UTF-8
			// byte sequence in the first position of a line, I have serous
			// doubts as to whether they really want ASA carriage control
			// treatment overall and maybe they should reconsider their life
			// choices (or at least reconsider their input file), but again,
			// we'll be generous and just ignore it and try to carry on
			// assuming this is a regular line.
			log.Printf("ERROR: invalid UTF-8 byte sequence at beginning "+
				"of line %d", linenum)
			control = rune(' ')
		}
		rest := line[size:]

		// If this is the first line, it may start with a "1" carriage control
		// to advance the printer to the beginning of the page. We're already
		// at the beginning of a new page, so we'll just prime prevLine with
		// the text of the current line and ignore the 1. Regular " " control
		// is also easy to handle here. '+' is meaningless since we don't have
		// a previous line to overstrike.
		//
		// We will also allow for the case where the first instruction is to
		// skip 1 or 2 lines.
		if linenum == 1 {
			switch control {
			case ' ', '1', '+','8','Q','3','R':
				// no special handling
                formline = 0
			case '0':
				handler.AddLine("", true)
                formline++
			case '-','7':
				handler.AddLine("", true)
				handler.AddLine("", true)
                formline += 2
			default:
				log.Printf("ERROR: unknown/unimplemented control "+
					"character '%s' on line %d", string(control), linenum)
			}
			prevline = rest
			continue
		}

		// Before sending the previous line to the printer, we need to read
		// this line's carriage control character so we know whether to tell
		// the printer to perform a line feed after printing the previous
		// line or if we'll overstrike.
		switch control {
		case ' ':
			handler.AddLine(prevline, true)
		case '1','8':
			handler.AddLine(prevline, true)
			handler.PageBreak()
            formline = 0
		case '0':
			handler.AddLine(prevline, true)
			handler.AddLine("", true)
            formline++
        case '2': //skip to end of form -2 lines
			handler.AddLine(prevline, true)
            jumptoline(64)
        case '3': //page eject
            if(linenum > 2) { //ignore page eject on first page
            handler.AddLine(prevline,true)
            handler.PageBreak()
            handler.PageBreak()
            formline = 0
            } else { formline-- }
        case '4': //skip 5 lines
            handler.AddLine(prevline,true)
            jumptoline(formline + 5)
        case '5': //skip 4 lines
			handler.AddLine(prevline, true)
            jumptoline(formline + 4)
        case '6': //skip 3 lines
			handler.AddLine(prevline, true)
            jumptoline(formline + 3)
		case '-','7':
			handler.AddLine(prevline, true)
			handler.AddLine("", true)
			handler.AddLine("", true)
            formline += 2
		case '+':
			handler.AddLine(prevline, false)
            formline--
		default:
			handler.AddLine(prevline, true)
			log.Printf("ERROR: unknown/unimplemented control "+
				"character '%s' on line %d", string(control), linenum)
		}

		// The line we just scanned becomes the new previous line for the next
		// iteration through the loop.
		prevline = rest
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// We always need to finish by writing the last line in the prevline
	// buffer
	handler.AddLine(prevline, true)
	handler.EndOfJob(jobname)

	return nil
}
