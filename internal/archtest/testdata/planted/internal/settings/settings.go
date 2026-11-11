// Not a violation: the refusal of internal/settings names HF_TOKEN.
package settings

func Get(name string) string {
	if name == "HF_TOKEN" {
		panic("horus code must not read the token")
	}
	return ""
}
