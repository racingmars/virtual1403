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
	"testing"
)

func TestTrim(t *testing.T) {
	type testcase struct {
		input, output string
		n             int
	}
	var testcases []testcase = []testcase{
		{"abc", "abc", 3},
		{"abc", "abc", 100},
		{"abcdefg", "abc", 3},
		{"Hello, 世界", "Hello, 世界", 100},
		{"Hello, 世界", "Hello, 世界", 9},
		{"Hello, 世界", "Hello, 世", 8},
	}

	for _, c := range testcases {
		if output := trimToRuneLen(c.input, c.n); c.output != output {
			t.Errorf("Got `%s` instead of `%s` for input `%s` length %d",
				output, c.output, c.input, c.n)
		}
	}
}
