package main

import (
	"fmt"
	"os"
)

func main() {
	restore, err := enableRawMode()
	if err != nil {
		fmt.Fprintf(os.Stderr, "enable raw mode: %v\n", err)
		os.Exit(1)
	}
	defer restore()

	fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H")
	fmt.Fprintln(os.Stdout, "tuirec testapp ready")
	fmt.Fprintln(os.Stdout, "Press Ctrl+Q to quit.")

	x := 10
	y := 5
	drawCursor(x, y, '#')

	buffer := make([]byte, 1)
	for {
		readByte(buffer)

		if buffer[0] == 0x11 {
			fmt.Fprintln(os.Stdout, "\r\nbye")

			return
		}

		if buffer[0] == '\x1b' {
			next, ok := readEscapeSequence(buffer)
			if ok {
				drawCursor(x, y, ' ')
				x, y = moveCursor(x, y, next)
				drawCursor(x, y, '#')
			}

			continue
		}

		if buffer[0] >= 0x20 && buffer[0] <= 0x7e {
			drawCursor(x, y, rune(buffer[0]))
			x++
			drawCursor(x, y, '#')
		}
	}
}

func readByte(buffer []byte) {
	for {
		n, err := os.Stdin.Read(buffer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
			os.Exit(1)
		}

		if n > 0 {
			return
		}
	}
}

func readEscapeSequence(buffer []byte) (byte, bool) {
	readByte(buffer)
	if buffer[0] != '[' {
		return 0, false
	}

	readByte(buffer)

	return buffer[0], true
}

func moveCursor(x, y int, direction byte) (int, int) {
	switch direction {
	case 'A':
		if y > 4 {
			y--
		}
	case 'B':
		y++
	case 'C':
		x++
	case 'D':
		if x > 1 {
			x--
		}
	}

	return x, y
}

func drawCursor(x, y int, r rune) {
	fmt.Fprintf(os.Stdout, "\x1b[%d;%dH%c", y, x, r)
}
