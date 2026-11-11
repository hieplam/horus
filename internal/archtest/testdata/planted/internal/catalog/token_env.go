// Planted: a string reads HF_TOKEN from a line of a .env file.
package catalog

import "strings"

func isToken(line string) bool { return strings.HasPrefix(line, "HF_TOKEN=") }
