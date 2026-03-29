package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

type Choice int

const (
	Accept Choice = iota
	Reject
	Edit
)

func Confirm(stderr io.Writer, stdin *os.File) (Choice, error) {
	if !term.IsTerminal(int(stdin.Fd())) {
		return Reject, nil
	}

	scanner := bufio.NewScanner(stdin)
	for {
		fmt.Fprint(stderr, "Commit? (y/n/e): ")
		if !scanner.Scan() {
			return Reject, scanner.Err()
		}

		switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
		case "y", "yes":
			return Accept, nil
		case "n", "no":
			return Reject, nil
		case "e", "edit":
			return Edit, nil
		default:
			fmt.Fprintln(stderr, "Please enter y (yes), n (no), or e (edit)")
		}
	}
}
