package mapper

import (
	"fmt"
	"strconv"
	"strings"
)

// Annotate renders an annotated hex dump of payload (16 bytes per line).
// annotations maps hex-offset strings (e.g. "0x01A3") or decimal strings to
// human-readable labels. Known offsets are labelled inline after the hex bytes.
func Annotate(payload []byte, annotations map[string]string) string {
	labels := parseAnnotations(annotations)

	var sb strings.Builder
	for i := 0; i < len(payload); i += 16 {
		end := min(i+16, len(payload))
		fmt.Fprintf(&sb, "0x%04X:  ", i)
		for j := i; j < end; j++ {
			fmt.Fprintf(&sb, "%02X ", payload[j])
		}
		// Pad short last line to align labels.
		for j := end; j < i+16; j++ {
			sb.WriteString("   ")
		}
		var marks []string
		for j := i; j < end; j++ {
			if lbl, ok := labels[j]; ok {
				marks = append(marks, fmt.Sprintf("0x%04X=%s", j, lbl))
			}
		}
		if len(marks) > 0 {
			sb.WriteString(" ; ")
			sb.WriteString(strings.Join(marks, ", "))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func parseAnnotations(raw map[string]string) map[int]string {
	out := make(map[int]string, len(raw))
	for k, v := range raw {
		n, err := strconv.ParseInt(strings.TrimSpace(k), 0, 64)
		if err == nil {
			out[int(n)] = v
		}
	}
	return out
}
