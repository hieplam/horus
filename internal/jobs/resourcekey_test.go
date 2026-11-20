package jobs

import (
	"regexp"
	"testing"
)

func TestResourceKey(t *testing.T) {
	hex64 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	a := ResourceKey("one", []string{"bb", "AA"})
	if !hex64.MatchString(a) {
		t.Fatalf("key %q is not 64 hex characters", a)
	}
	if b := ResourceKey("two", []string{"aa", "bb"}); a != b {
		t.Errorf("the same model files in another order and another workflow give another key: %s and %s", a, b)
	}
	if c := ResourceKey("one", []string{"aa", "cc"}); a == c {
		t.Errorf("other model files give the same key %s", a)
	}
	if x, y := ResourceKey("one", nil), ResourceKey("two", nil); x == y || !hex64.MatchString(x) {
		t.Errorf("workflows without model files: keys %s and %s, want two different keys of 64 hex characters", x, y)
	}
}
