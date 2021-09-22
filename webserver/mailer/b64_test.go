package mailer

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
	"bytes"
	"encoding/base64"
	"testing"
)

func TestB64(t *testing.T) {
	input := "This is a string that is long and should need to wrap a few lines of Base64 to fit in the prescribed width."
	var buf bytes.Buffer
	if err := wrappedBase64([]byte(input), &buf); err != nil {
		t.Fail()
	}
	// 5 extra bytes to make sure we don't get more back than we encoded
	output := make([]byte, len(input)+5)
	n, err := base64.StdEncoding.Decode(output, buf.Bytes())
	if err != nil {
		t.Fail()
	}
	if n != len(input) {
		t.Error("Got back different number of bytes")
	}
	if input != string(output[:n]) {
		t.Error("Got back different output")
	}
}
