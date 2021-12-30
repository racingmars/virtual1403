package main

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
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/racingmars/virtual1403/scanner"
	"github.com/racingmars/virtual1403/vprinter"
)

type pdfOutputHandler struct {
	job       vprinter.Job
	outputDir string
	font      []byte
}

func newPDFOutputHandler(outputDir string, font []byte) (
	scanner.PrinterHandler, error) {

	o := &pdfOutputHandler{
		outputDir: outputDir,
		font:      font,
	}
	var err error

	o.job, err = vprinter.New1403(o.font, 11.4, 5, true, true,
		vprinter.DarkGreen, vprinter.LightGreen)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (o *pdfOutputHandler) AddLine(line string, linefeed bool) {
	o.job.AddLine(line, linefeed)
}

func (o *pdfOutputHandler) PageBreak() {
	o.job.NewPage()
}

func (o *pdfOutputHandler) EndOfJob(jobinfo string) {
	// No matter what happens, we always want to reset our state to a fresh
	// new job.
	defer func() {
		var err error
		o.job, err = vprinter.New1403(o.font, 11.4, 5, true, true,
			vprinter.DarkGreen, vprinter.LightGreen)
		if err != nil {
			log.Printf("ERROR: couldn't re-initialize virtual 1403: %v\n",
				err)
			log.Printf(
				"ERROR: application is probably in a bad state, please restart.\n")
		}
	}()

	if jobinfo != "" {
		jobinfo = jobinfo + "-"
	}
	jobfilename := fmt.Sprintf("v1403-%s%s.pdf", jobinfo,
		time.Now().UTC().Format("20060102T030405"))
	filename := filepath.Join(o.outputDir, jobfilename)

	f, err := os.Create(filename)
	if err != nil {
		log.Printf("ERROR: couldn't create output file: %v\n", err)
		return
	}
	defer f.Close()
	n, err := o.job.EndJob(f)
	if err != nil {
		log.Printf("ERROR: couldn't write PDF output: %v\n", err)
		return
	}

	log.Printf("INFO:  wrote %d page PDF to %s\n", n, filename)
}
