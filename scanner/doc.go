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

/*
Package scanner is a tiny combined lexer+parser that emits lines of printer
output, page breaks, and end-of-job indications. It specifically handles
reading from Hercules sockdev printer output of JES2 jobs in MVS 3.8J,
although if there is a reliable way to detect the last line of a job (which
must immediately be followed by a form feed character) with a regular
expression, the code can be easily adapted for other operating systems.

We tolerate a variety of combinations of CR, LF, and FF. A bare CR will cause
the next line to overtype the current line. Bare LF, CR+LF, or LF+CR have the
effect of CR+LF.

Lines will be trimmed to 132 bytes; additional bytes on a line will be
discarded.

The implementation is a state machine that reads one byte at a time, updates
internal state as necessary, performs actions (emit lines, pages, EOJ) as
necessary, then sets the function to handle the next byte. Thus the current
state of the state machine is represented by the value of the state function
on the scanner struct.

The state machine is approximately an implementation of the following diagram
in the dot language:

	digraph {
		"get next byte" -> "have lf" [label="b=lf"];
		"get next byte" -> "have cr" [label="b=cr"];
		"get next byte" -> "emit line and page" [label="b=ff"];
		"emit line and page" [shape=box];
		"emit line and page" -> "last line match EOJ pattern?";
		"last line match EOJ pattern?" [shape=box];
		"last line match EOJ pattern?" -> "emit EOJ" [label="yes"];
		"last line match EOJ pattern?" -> "get next byte" [label="no"];
		"have lf" -> "emit line" [label="b=cr"];
		"have lf" -> "emit line and await lf" [label="b=lf"];
		"have lf" -> "emit line and page" [label="b=ff"];
		"have lf" -> "get next byte" [label="b != lf | cr | ff"];
		"emit line and await lf" [shape=box];
		"emit line and await lf" -> "have lf";
		"have cr" -> "emit line" [label="b=lf"];
		"have cr" -> "emit line and await cr" [label="b=cr"];
		"have cr" -> "emit line and page" [label="b=ff"];
		"have cr" -> "get next byte" [label="b != lf | cr | ff"];
		"emit line and await cr" [shape=box];
		"emit line and await cr" -> "have cr";
		"emit line" [shape=box];
		"emit line" -> "get next byte";
		"get next byte" -> "add to current line";
		"add to current line" [shape=box];
		"add to current line" -> "dispose of bytes" [label="n>=132"];
		"add to current line" -> "get next byte" [label="n<132"];
		"dispose of bytes" -> "have lf" [label="b=lf"];
		"dispose of bytes" -> "have cr" [label="b=cr"];
		"dispose of bytes" -> "emit line and page" [label="b=ff"];
		"dispose of bytes" -> "dispose of bytes" [label="b != lf | cr | ff"];
		"emit EOJ" [shape=box];
		"emit EOJ" -> "get next byte";
	}

Is this all slightly over-complicated for our needs? Perhaps.
*/
package scanner
