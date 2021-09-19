package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/racingmars/virtual1403/vprinter"
)

type outputHandler struct {
	job       vprinter.Job
	outputDir string
	font      []byte
}

func newOutputHandler(outputDir string, font []byte) (*outputHandler, error) {
	o := &outputHandler{
		outputDir: outputDir,
		font:      font,
	}
	var err error

	o.job, err = vprinter.New1403(o.font)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (o *outputHandler) AddLine(line string) {
	o.job.AddLine(line)
}

func (o *outputHandler) PageBreak() {
	o.job.NewPage()
}

func (o *outputHandler) EndOfJob() {
	// No matter what happens, we always want to reset our state to a fresh
	// new job.
	defer func() {
		var err error
		o.job, err = vprinter.New1403(o.font)
		if err != nil {
			log.Printf("ERROR: couldn't re-initialize virtual 1403: %v\n", err)
			log.Printf("ERROR: application is probably in a bad state, please restart.\n")
		}
	}()

	filename := filepath.Join(o.outputDir, time.Now().UTC().Format("20060102T030405.pdf"))

	f, err := os.Create(filename)
	if err != nil {
		log.Printf("ERROR: couldn't create output file: %v\n", err)
		return
	}
	defer f.Close()
	err = o.job.EndJob(f)
	if err != nil {
		log.Printf("ERROR: couldn't write PDF output: %v\n", err)
		return
	}

	log.Printf("INFO:  wrote PDF to %s\n", filename)
}
