package witness

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/mod/sumdb/note"
)

// MergeCosignatures combines an operator-signed checkpoint with additional cosigned
// checkpoints, returning a single note that contains all unique signatures.
// The signed note format is: <text>\n\n<sigline1>\n<sigline2>\n...
func MergeCosignatures(operatorSigned []byte, cosignedNotes []string, _ note.Verifier) ([]byte, error) {
	// Split the operator-signed note into body and signature sections.
	idx := bytes.Index(operatorSigned, []byte("\n\n"))
	if idx < 0 {
		return nil, fmt.Errorf("malformed operator note: missing \\n\\n separator")
	}

	body := string(operatorSigned[:idx+1]) // body including its trailing \n
	sigSection := string(operatorSigned[idx+2:])

	// Collect unique signature lines from the operator note.
	seen := map[string]bool{}
	var sigLines []string
	for _, line := range strings.Split(sigSection, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || !strings.HasPrefix(line, "— ") {
			continue
		}
		if !seen[line] {
			seen[line] = true
			sigLines = append(sigLines, line)
		}
	}

	// Collect unique sig lines from each cosigned note.
	for _, cosigned := range cosignedNotes {
		parts := strings.SplitN(cosigned, "\n\n", 2)
		if len(parts) < 2 {
			continue
		}
		for _, line := range strings.Split(parts[1], "\n") {
			line = strings.TrimRight(line, "\r")
			if line == "" || !strings.HasPrefix(line, "— ") {
				continue
			}
			if !seen[line] {
				seen[line] = true
				sigLines = append(sigLines, line)
			}
		}
	}

	// Reassemble the merged note: body + \n + sig lines.
	var out strings.Builder
	out.WriteString(body)
	out.WriteString("\n")
	for _, line := range sigLines {
		out.WriteString(line)
		out.WriteString("\n")
	}

	return []byte(out.String()), nil
}

// ExtractCosignatures extracts signature lines from a cosigned note (excluding the operator).
func ExtractCosignatures(cosigned string, operatorName string) []string {
	parts := strings.SplitN(cosigned, "\n\n", 2)
	if len(parts) < 2 {
		return nil
	}
	var out []string
	for _, line := range strings.Split(parts[1], "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "— ") {
			continue
		}
		if strings.HasPrefix(line[2:], operatorName+" ") {
			continue
		}
		out = append(out, line)
	}
	return out
}
