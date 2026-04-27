package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// isBareFileFlag reports true when args end with `-f` or `--file` having
// no value after it. Cobra would otherwise produce a confusing exit-1
// error; we want exit-2 with apfel's wording.
func isBareFileFlag(args []string) bool {
	if len(args) == 0 {
		return false
	}
	last := args[len(args)-1]
	return last == "-f" || last == "--file"
}

// readFileFlags resolves -f/--file arguments to their text content.
// Mirrors apfel's behaviour:
//   - empty path     -> exit 2 "requires a file path"
//   - nonexistent    -> exit 2 "no such file"
//   - JPEG/PNG image -> exit 2 "text-only ... images not supported"
//   - non-UTF-8 / binary content -> exit 2 "binary ... text-only"
//   - multiple -f flags concatenate (separated by blank lines).
func readFileFlags(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}
	var sb strings.Builder
	for _, p := range paths {
		if p == "" {
			return "", &exitError{code: exitInvalidArgs, msg: "-f/--file requires a file path"}
		}
		body, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				return "", &exitError{code: exitInvalidArgs, msg: fmt.Sprintf("no such file: %s", p)}
			}
			return "", &exitError{code: exitInvalidArgs, msg: fmt.Sprintf("cannot read file %s: %v", p, err)}
		}
		// Image header? Reject with text-only message.
		if isImage(body) {
			return "", &exitError{code: exitInvalidArgs, msg: fmt.Sprintf("file %s appears to be an image; fenster is text-only and does not support image input", p)}
		}
		// Non-UTF-8 / binary content?
		if !utf8.Valid(body) {
			return "", &exitError{code: exitInvalidArgs, msg: fmt.Sprintf("file %s appears to be binary or not UTF-8 text; fenster only accepts text-only input", p)}
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.Write(body)
	}
	return sb.String(), nil
}

// isImage detects common image-file headers (JPEG/PNG/GIF/WEBP/BMP).
func isImage(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	switch {
	case b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff: // JPEG
		return true
	case b[0] == 0x89 && string(b[1:4]) == "PNG":
		return true
	case len(b) >= 6 && string(b[:6]) == "GIF87a":
		return true
	case len(b) >= 6 && string(b[:6]) == "GIF89a":
		return true
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return true
	case b[0] == 'B' && b[1] == 'M': // BMP
		return true
	}
	return false
}

// combinePromptAndFiles returns the final prompt: file contents (if any),
// then a blank line, then the user's positional prompt.
func combinePromptAndFiles(files string, prompt string) (string, error) {
	files = strings.TrimRight(files, "\n")
	prompt = strings.TrimSpace(prompt)
	switch {
	case files != "" && prompt != "":
		return files + "\n\n" + prompt, nil
	case files != "":
		return files, nil
	case prompt != "":
		return prompt, nil
	default:
		return "", errors.New("no prompt or file provided")
	}
}
