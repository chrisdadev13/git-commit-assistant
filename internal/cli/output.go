package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

const (
	colorReset = "\033[0m"
	colorBold  = "\033[1m"
	colorDim   = "\033[2m"
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
)

type Formatter struct {
	Color  bool
	Stdout io.Writer
	Stderr io.Writer
}

func NewFormatter(stdout, stderr *os.File) *Formatter {
	color := term.IsTerminal(int(stderr.Fd())) && os.Getenv("NO_COLOR") == ""
	return &Formatter{
		Color:  color,
		Stdout: stdout,
		Stderr: stderr,
	}
}

func (f *Formatter) style(code, text string) string {
	if !f.Color {
		return text
	}
	return code + text + colorReset
}

func (f *Formatter) Status(msg string) {
	fmt.Fprintln(f.Stderr, f.style(colorDim, msg))
}

func (f *Formatter) Success(msg string) {
	fmt.Fprintln(f.Stderr, f.style(colorGreen, msg))
}

func (f *Formatter) Error(msg string) {
	fmt.Fprintln(f.Stderr, f.style(colorRed, msg))
}

func (f *Formatter) CommitMessage(message string) {
	sep := "----------------------------------"
	fmt.Fprintln(f.Stderr, f.style(colorDim, sep))
	fmt.Fprintln(f.Stderr, f.style(colorBold, message))
	fmt.Fprintln(f.Stderr, f.style(colorDim, sep))
}

func (f *Formatter) JSON(message string) error {
	out := struct {
		Commit string `json:"commit"`
	}{Commit: message}

	enc := json.NewEncoder(f.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func (f *Formatter) Plain(message string) {
	fmt.Fprintln(f.Stdout, message)
}
