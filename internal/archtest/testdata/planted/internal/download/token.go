// Not a violation: internal/download reads the token.
package download

import "os"

var token = os.Getenv("HF_TOKEN")
