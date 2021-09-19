package vprinter

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
	"fmt"
	"io"
	"math"
	"os"

	"github.com/jung-kurt/gofpdf"
)

// Job is the interface that each virtual printers must implement to receive
// print jobs.
type Job interface {
	// AddLine adds one line of output to the print job. linefeed indicates
	// whether, in addition to a carriage return action at the end of the
	// line, the printer should advance by one line. (linefeed=false will
	// cause the next line to overtype the current line.)
	AddLine(text string, linefeed bool)

	// NewPage indicates that the print job encountered a form feed character.
	NewPage()

	// EndJob instructs the virtual printer to end the job and write the
	// output (e.g. the PDF of all lines and pages for this job) to the
	// io.Writer.
	EndJob(io.Writer) error
}

// LoadFont will load a font file from path, verify that it is usable with the
// gofpdf library, and that it is a fixed-with font. If everything is okay,
// we will return the font as a byte array and error will be nil.
func LoadFont(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("couldn't open font file: %v", err)
	}
	defer f.Close()

	fontdata, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("couldn't read font file: %v", err)
	}

	if err = makeTestPDF(fontdata); err != nil {
		return nil, fmt.Errorf("couldn't use font: %v", err)
	}

	if err = verifyFixedWidth(fontdata); err != nil {
		return nil, fmt.Errorf("font is not fixed-width")
	}

	return fontdata, nil
}

func makeTestPDF(font []byte) error {
	pdf := gofpdf.New(gofpdf.OrientationPortrait, "pt",
		gofpdf.PageSizeLetter, "")
	pdf.AddUTF8FontFromBytes("userfont", "", font)
	pdf.AddPage()
	pdf.SetFont("userfont", "", 12)
	if err := pdf.Output(io.Discard); err != nil {
		return err
	}

	return nil
}

func verifyFixedWidth(font []byte) error {
	pdf := gofpdf.New(gofpdf.OrientationPortrait, "pt",
		gofpdf.PageSizeLetter, "")
	pdf.AddUTF8FontFromBytes("userfont", "", font)
	pdf.SetFont("userfont", "", 12)
	size1 := pdf.GetStringWidth("llllllllllllllllllll")
	size2 := pdf.GetStringWidth("OOOOOOOOOOOOOOOOOOOO")
	const epsilon = 1.0 // allow 1 pt line length difference
	if math.Abs(size1-size2) > epsilon {
		return fmt.Errorf("font is not fixed width")
	}
	return nil
}
