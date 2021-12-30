package main

import (
	"fmt"
	"io"
	"os"

	"github.com/racingmars/virtual1403/vprinter"
)

func main() {
	font, err := loadFontData("../../../Downloads/IBM140310Pitch-Regular-MRW.ttf")
	if err != nil {
		panic(err)
	}
	printer, err := vprinter.New1403(font, 10, 5, true, false,
		vprinter.DarkGreen, vprinter.LightGreen)
	if err != nil {
		panic(err)
	}

	printer.AddLine("**** HELLO WORLD. TESTING 1...2...3... ****", true)
	printer.AddLine("000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000111111111111111111111111111111111", true)
	printer.AddLine("000000000111111111122222222223333333333444444444455555555556666666666777777777788888888889999999999000000000011111111112222222222333", true)
	printer.AddLine("123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012", true)
	printer.AddLine("", true)
	printer.AddLine("000000 <-- THESE ZEROS WILL BE OVERTYPED WITH SLASHES (/)", false)
	printer.AddLine("//////", true)
	printer.AddLine("", true)
	printer.AddLine("THE FOLLOWING LINE MAKES BOLD TEXT BY OVERSTRIKING THE LINE WITH THE SAME TEXT:", true)
	printer.AddLine("THIS IS BOLD TEXT AND THIS IS REGULAR TEXT.", false)
	printer.AddLine("THIS IS BOLD TEXT", true)
	printer.AddLine("", true)
	printer.AddLine("FORM FEED FOLLOWS THIS LINE", true)

	printer.NewPage()

	printer.AddLine("NOW GOING TO SIMULATE \"PRINTING\" 150 LINES WITH NO FORM FEED:", true)
	for i := 0; i < 150; i++ {
		printer.AddLine(fmt.Sprintf("**** LINE %03d OF 150 ****  ABCDEFGHIJKLMNOPQRSTUVWXYZ 0123456789", i+1), true)
	}

	f, err := os.Create("hello.pdf")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if _, err = printer.EndJob(f); err != nil {
		panic(err)
	}
}

func loadFontData(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return data, nil
}
