Virtual 1403
============

Getting started
---------------

If building from source, from this agent directory, simply run `go build -o virtual1403` to get an executable called virtual1403. Go 1.16 or later is required to build.

Create a `config.yaml` file with the address of your Hercules sockdev printer device, the output directory for PDFs, and optionally a font file to use. There are some restrictions on the fonts that are supported: they must be fixed-width fonts, and the font file must use the TrueType outline format (see _Font conversion notes_ below).

If you do not provide a font file, virtual1403 includes an embedded copy of IBM Plex Mono that it will use.

Example config.yaml file:

```
hercules_address: "127.0.0.1:1403"
output_directory: "pdfs"
# Optional font file; delete line or comment out to use built-in font
font_file: "font-file-name.ttf"
```

Start virtual1403 and it should connect to Hercules and process printer output, creating a timestamped PDF file in the output directory for each job. Currently output from MVS 3.8 is supported; job separation has been tested with TK4- and Jay Moseley's sysgen.

Font conversion notes
---------------------

If you receive error messages when trying to load your custom font, you can try to convert it to a format supported by the PDF library using FontForge. Ensure the fontforge command is available on your system, and create the following script (<http://www.stuermer.ch/blog/convert-otf-to-ttf-font-on-ubuntu.html>):

```
#!/usr/bin/fontforge
# Quick and dirty hack: converts a font to truetype (.ttf)
Print("Opening "+$1);
Open($1);
Print("Saving "+$1:r+".out.ttf");
Generate($1:r+".out.ttf");
Quit(0); 
```

Make the script executable (`chmod +x convertfont.sh`), and run it on the font file (`./convertfont.sh my-font-file.otf`) to create a TrueType version that should work.

Acknowledgements
----------------

virtual1403 is developed in collaboration with Moshix (https://github.com/moshix/), and the greenbar paper design is based on photos he provided of real 1403 printouts he has.

License
-------

    Copyright 2021 Matthew R. Wilson <mwilson@mattwilson.org>

    This file is part of virtual1403
    <https://github.com/racingmars/virtual1403>.

    virtual1403 is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    virtual1403 is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with virtual1403. If not, see <https://www.gnu.org/licenses/>.


Third-party licenses
--------------------

The IBM Plex Mono font is embedded in the application, and is distributed under the SIL Open Font License; see the file IBMPlexMono-Regular.license for details.

PDF generation is performed using the gofpdf library from <https://github.com/jung-kurt/gofpdf>, and is provided under the MIT license:

    Copyright (c) 2017 Kurt Jung and contributors acknowledged in the documentation

    Permission is hereby granted, free of charge, to any person obtaining a copy
    of this software and associated documentation files (the "Software"), to deal
    in the Software without restriction, including without limitation the rights
    to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
    copies of the Software, and to permit persons to whom the Software is
    furnished to do so, subject to the following conditions:

    The above copyright notice and this permission notice shall be included in all
    copies or substantial portions of the Software.

    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
    AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
    OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
    SOFTWARE.

yaml.v3 is included, and is provided under the following licenses:

    #### MIT License ####

    The following files were ported to Go from C files of libyaml, and thus
    are still covered by their original MIT license, with the additional
    copyright staring in 2011 when the project was ported over:

        apic.go emitterc.go parserc.go readerc.go scannerc.go
        writerc.go yamlh.go yamlprivateh.go

    Copyright (c) 2006-2010 Kirill Simonov
    Copyright (c) 2006-2011 Kirill Simonov

    Permission is hereby granted, free of charge, to any person obtaining a copy of
    this software and associated documentation files (the "Software"), to deal in
    the Software without restriction, including without limitation the rights to
    use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
    of the Software, and to permit persons to whom the Software is furnished to do
    so, subject to the following conditions:

    The above copyright notice and this permission notice shall be included in all
    copies or substantial portions of the Software.

    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
    AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
    OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
    SOFTWARE.

    ### Apache License ###

    All the remaining project files are covered by the Apache license:

    Copyright (c) 2011-2019 Canonical Ltd

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
