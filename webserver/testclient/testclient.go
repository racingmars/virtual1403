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

// A simple test client for posting print jobs to the web service.

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/klauspost/compress/zstd"
)

func main() {
	var b bytes.Buffer
	enc, _ := zstd.NewWriter(&b)
	w := bufio.NewWriter(enc)
	w.WriteString("L:This is a test.\n")
	w.WriteString("L:This is another test.\n")
	w.WriteString("P:\n")
	w.WriteString("O:0000 new page\n")
	w.WriteString("L:////\n")
	w.WriteString("L:Last line.\n")
	w.Flush()
	enc.Close()

	req, err := http.NewRequest(http.MethodPost,
		"http://localhost:4444/print", &b)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Encoding", "zstd")
	req.Header.Set("Content-Type", "text/x-print-job")
	req.Header.Set("Authorization", "Bearer yYb+9XgZ4PpNKFQKOjNf+TS5q67WYdL81hnM0H3k/7E=")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %s\n", resp.Status)
	io.Copy(os.Stdout, resp.Body)
}
