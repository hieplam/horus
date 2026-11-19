package jobs

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// The two workflow revisions of the vectors: WorkflowRev of a small manifest
// and graph (TestWorkflowRev checks them).
const (
	revA = "dd1d52408eb4e198491c6226f216f36cff1d362059f7dfcc314e0b6aa16a3c79"
	revB = "f558df07ffd5c763567e9ee1e174ed45f2d0d918873296504a9ecb267de2b573"
)

func u64(v uint64) *uint64 { return &v }
func i64(v int64) *int64   { return &v }

// base is the request of the vectors. Each vector changes one part.
func base() Request {
	return Request{Tool: "generate_image", Workflow: "it-illustration", WorkflowRev: revA,
		Subject: "A flat vector illustration: a message queue & 3 laptops", DefaultWidth: 1024, DefaultHeight: 1024}
}

func with(f func(r *Request)) Request {
	r := base()
	f(&r)
	return r
}

// vector is one fixed vector of recipe version 1: the input, the exact JSON
// text, its SHA-256 and the key. The Go code is the source of the values
// (ADR 0027): a change of a value is a change of the recipe, and needs a new
// version.
type vector struct {
	name string
	in   Request
	text string
	sha  string
	key  string
}

var vectors = []vector{
	{
		name: "a seed",
		in:   with(func(r *Request) { r.Seed = u64(42) }),
		text: `{"v":1,"tool":"generate_image","workflow":"it-illustration","workflow_rev":"dd1d52408eb4e198491c6226f216f36cff1d362059f7dfcc314e0b6aa16a3c79","subject":"A flat vector illustration: a message queue & 3 laptops","seed":"42","width":1024,"height":1024,"out_path":null,"overwrite":false}`,
		sha:  "9862d321cce2aff4a7a26ce430b886e5e13fa92897675491ee1c45b2a7f852d8",
		key:  "f1:9862d321cce2aff4a7a26ce430b886e5e13fa92897675491ee1c45b2a7f852d8",
	},
	{
		name: "no seed",
		in:   base(),
		text: `{"v":1,"tool":"generate_image","workflow":"it-illustration","workflow_rev":"dd1d52408eb4e198491c6226f216f36cff1d362059f7dfcc314e0b6aa16a3c79","subject":"A flat vector illustration: a message queue & 3 laptops","seed":null,"width":1024,"height":1024,"out_path":null,"overwrite":false}`,
		sha:  "e380da60b70dfb8fe276d38a3063d5337cfb8b0c4cf6ec05f02fdcf878c8bcb9",
		key:  "",
	},
	{
		name: "a caller key",
		in:   with(func(r *Request) { r.Seed = u64(42); r.CallerKey = "blog-header-7" }),
		text: `{"v":1,"tool":"generate_image","workflow":"it-illustration","workflow_rev":"dd1d52408eb4e198491c6226f216f36cff1d362059f7dfcc314e0b6aa16a3c79","subject":"A flat vector illustration: a message queue & 3 laptops","seed":"42","width":1024,"height":1024,"out_path":null,"overwrite":false}`,
		sha:  "9862d321cce2aff4a7a26ce430b886e5e13fa92897675491ee1c45b2a7f852d8",
		key:  "k1:9afca5d3ea9f66d40dd9a6d9855e76b8876b01a340524fc932177341b921b00e",
	},
	{
		name: "spaces in the subject",
		in: with(func(r *Request) {
			r.Seed = u64(42)
			r.Subject = "  A flat   vector illustration:\ta message queue\n& 3 laptops  "
		}),
		text: `{"v":1,"tool":"generate_image","workflow":"it-illustration","workflow_rev":"dd1d52408eb4e198491c6226f216f36cff1d362059f7dfcc314e0b6aa16a3c79","subject":"A flat vector illustration: a message queue & 3 laptops","seed":"42","width":1024,"height":1024,"out_path":null,"overwrite":false}`,
		sha:  "9862d321cce2aff4a7a26ce430b886e5e13fa92897675491ee1c45b2a7f852d8",
		key:  "f1:9862d321cce2aff4a7a26ce430b886e5e13fa92897675491ee1c45b2a7f852d8",
	},
	{
		name: "the default size",
		in:   with(func(r *Request) { r.Seed = u64(7) }),
		text: `{"v":1,"tool":"generate_image","workflow":"it-illustration","workflow_rev":"dd1d52408eb4e198491c6226f216f36cff1d362059f7dfcc314e0b6aa16a3c79","subject":"A flat vector illustration: a message queue & 3 laptops","seed":"7","width":1024,"height":1024,"out_path":null,"overwrite":false}`,
		sha:  "10d2ac8055ee524ef0f952fefae47d85f671ab4524e0b11e33462dceec4f2336",
		key:  "f1:10d2ac8055ee524ef0f952fefae47d85f671ab4524e0b11e33462dceec4f2336",
	},
	{
		name: "the same size given",
		in:   with(func(r *Request) { r.Seed = u64(7); r.Width = i64(1024); r.Height = i64(1024) }),
		text: `{"v":1,"tool":"generate_image","workflow":"it-illustration","workflow_rev":"dd1d52408eb4e198491c6226f216f36cff1d362059f7dfcc314e0b6aa16a3c79","subject":"A flat vector illustration: a message queue & 3 laptops","seed":"7","width":1024,"height":1024,"out_path":null,"overwrite":false}`,
		sha:  "10d2ac8055ee524ef0f952fefae47d85f671ab4524e0b11e33462dceec4f2336",
		key:  "f1:10d2ac8055ee524ef0f952fefae47d85f671ab4524e0b11e33462dceec4f2336",
	},
	{
		name: "out_path on Windows",
		in: with(func(r *Request) {
			r.Seed = u64(42)
			r.OutPath = `C:\Users\agent\Pictures\..\Shared/Blog\Header.PNG`
			r.PathStyle = PathWindows
		}),
		text: `{"v":1,"tool":"generate_image","workflow":"it-illustration","workflow_rev":"dd1d52408eb4e198491c6226f216f36cff1d362059f7dfcc314e0b6aa16a3c79","subject":"A flat vector illustration: a message queue & 3 laptops","seed":"42","width":1024,"height":1024,"out_path":"c:\\users\\agent\\shared\\blog\\header.png","overwrite":false}`,
		sha:  "74ade219be9f5bdb5d5e512c085203ed0ae7e7fc74cb1bdc0ba6390c4f94a24d",
		key:  "f1:74ade219be9f5bdb5d5e512c085203ed0ae7e7fc74cb1bdc0ba6390c4f94a24d",
	},
	{
		name: "out_path on macOS",
		in: with(func(r *Request) {
			r.Seed = u64(42)
			r.OutPath = `/Users/agent/Shared//Blog/./Header.PNG`
			r.PathStyle = PathMacOS
		}),
		text: `{"v":1,"tool":"generate_image","workflow":"it-illustration","workflow_rev":"dd1d52408eb4e198491c6226f216f36cff1d362059f7dfcc314e0b6aa16a3c79","subject":"A flat vector illustration: a message queue & 3 laptops","seed":"42","width":1024,"height":1024,"out_path":"/users/agent/shared/blog/header.png","overwrite":false}`,
		sha:  "d3f11a9f1c7ae7b62667cdcc82eed80bf65ecf190b41bf14821f9c683f26268a",
		key:  "f1:d3f11a9f1c7ae7b62667cdcc82eed80bf65ecf190b41bf14821f9c683f26268a",
	},
	{
		name: "out_path on Linux",
		in: with(func(r *Request) {
			r.Seed = u64(42)
			r.OutPath = `/srv/shared/../shared/Blog/Header.PNG`
			r.PathStyle = PathLinux
		}),
		text: `{"v":1,"tool":"generate_image","workflow":"it-illustration","workflow_rev":"dd1d52408eb4e198491c6226f216f36cff1d362059f7dfcc314e0b6aa16a3c79","subject":"A flat vector illustration: a message queue & 3 laptops","seed":"42","width":1024,"height":1024,"out_path":"/srv/shared/Blog/Header.PNG","overwrite":false}`,
		sha:  "defefb06973d280bda32ad6a2f095fd2d6491c14d6c566ca227a8ed4d7a782ee",
		key:  "f1:defefb06973d280bda32ad6a2f095fd2d6491c14d6c566ca227a8ed4d7a782ee",
	},
	{
		name: "overwrite",
		in:   with(func(r *Request) { r.Seed = u64(42); r.Overwrite = true }),
		text: `{"v":1,"tool":"generate_image","workflow":"it-illustration","workflow_rev":"dd1d52408eb4e198491c6226f216f36cff1d362059f7dfcc314e0b6aa16a3c79","subject":"A flat vector illustration: a message queue & 3 laptops","seed":"42","width":1024,"height":1024,"out_path":null,"overwrite":true}`,
		sha:  "a79049002fd8e40d7b930f4bb066e0092e0ca8b595499b3e356672279ccf5fcc",
		key:  "f1:a79049002fd8e40d7b930f4bb066e0092e0ca8b595499b3e356672279ccf5fcc",
	},
	{
		name: "another workflow_rev",
		in:   with(func(r *Request) { r.Seed = u64(42); r.WorkflowRev = revB }),
		text: `{"v":1,"tool":"generate_image","workflow":"it-illustration","workflow_rev":"f558df07ffd5c763567e9ee1e174ed45f2d0d918873296504a9ecb267de2b573","subject":"A flat vector illustration: a message queue & 3 laptops","seed":"42","width":1024,"height":1024,"out_path":null,"overwrite":false}`,
		sha:  "cf365a4ae58f361d989a371961431e03637cf07a3c900297ceed3c895fa1dac9",
		key:  "f1:cf365a4ae58f361d989a371961431e03637cf07a3c900297ceed3c895fa1dac9",
	},
	{
		name: "the seed 2^64-1",
		in:   with(func(r *Request) { r.Seed = u64(18446744073709551615) }),
		text: `{"v":1,"tool":"generate_image","workflow":"it-illustration","workflow_rev":"dd1d52408eb4e198491c6226f216f36cff1d362059f7dfcc314e0b6aa16a3c79","subject":"A flat vector illustration: a message queue & 3 laptops","seed":"18446744073709551615","width":1024,"height":1024,"out_path":null,"overwrite":false}`,
		sha:  "3fb6f9581a7833b89a953ba278e835d54f52990ce187e59ce28eaca4380c4d25",
		key:  "f1:3fb6f9581a7833b89a953ba278e835d54f52990ce187e59ce28eaca4380c4d25",
	},
}

// TestRecipeVectors checks the 12 fixed vectors of recipe version 1. The goal
// row idempotency-recorded counts its 12 passing subtests.
func TestRecipeVectors(t *testing.T) {
	if len(vectors) != 12 {
		t.Fatalf("%d vectors, want 12", len(vectors))
	}
	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			// The SHA-256 of the fixed text, apart from the code under test.
			sum := sha256.Sum256([]byte(v.text))
			if hex.EncodeToString(sum[:]) != v.sha {
				t.Fatalf("the vector is wrong: SHA-256 of its text is %x, the vector says %s", sum, v.sha)
			}
			f, err := Recipe(v.in)
			if err != nil {
				t.Fatal(err)
			}
			if f.Text != v.text {
				t.Errorf("text:\n%s\nwant:\n%s", f.Text, v.text)
			}
			if f.Hash != v.sha {
				t.Errorf("hash %q, want %q", f.Hash, v.sha)
			}
			if f.Key != v.key {
				t.Errorf("key %q, want %q", f.Key, v.key)
			}
		})
	}
}

// TestRecipeRelations checks what the vectors mean together: the same request
// written two ways has one fingerprint, and each other change gives another.
func TestRecipeRelations(t *testing.T) {
	byName := map[string]vector{}
	for _, v := range vectors {
		byName[v.name] = v
	}
	same := [][2]string{
		{"a seed", "spaces in the subject"},
		{"the default size", "the same size given"},
		{"a seed", "a caller key"}, // the caller key changes the key, not the fingerprint
	}
	for _, p := range same {
		if byName[p[0]].sha != byName[p[1]].sha {
			t.Errorf("%q and %q must have one fingerprint", p[0], p[1])
		}
	}
	for _, other := range []string{"no seed", "out_path on Windows", "out_path on macOS", "out_path on Linux",
		"overwrite", "another workflow_rev", "the seed 2^64-1", "the default size"} {
		if byName[other].sha == byName["a seed"].sha {
			t.Errorf("%q must have another fingerprint than %q", other, "a seed")
		}
	}
	for _, v := range vectors {
		switch {
		case v.in.CallerKey != "":
			if !strings.HasPrefix(v.key, "k1:") {
				t.Errorf("%s: a caller key gives k1:, got %q", v.name, v.key)
			}
		case v.in.Seed != nil:
			if v.key != "f1:"+v.sha {
				t.Errorf("%s: a seed gives f1: and the fingerprint, got %q", v.name, v.key)
			}
		default:
			if v.key != "" {
				t.Errorf("%s: no seed and no caller key gives no key, got %q", v.name, v.key)
			}
		}
	}
}

func TestWorkflowRev(t *testing.T) {
	if got := WorkflowRev([]byte(`{"name":"it-illustration"}`), []byte(`{"nodes":[]}`)); got != revA {
		t.Errorf("revision A %s, want %s", got, revA)
	}
	if got := WorkflowRev([]byte(`{"name":"it-illustration","version":2}`), []byte(`{"nodes":[]}`)); got != revB {
		t.Errorf("revision B %s, want %s", got, revB)
	}
	// The 0 byte keeps the two files apart: "ab"+"c" is not "a"+"bc".
	if WorkflowRev([]byte("ab"), []byte("c")) == WorkflowRev([]byte("a"), []byte("bc")) {
		t.Error("the revision does not keep the two files apart")
	}
}

func TestCleanPath(t *testing.T) {
	for _, c := range []struct {
		in    string
		style PathStyle
		want  string
	}{
		{`C:\Users\A\..\B\x.PNG`, PathWindows, `c:\users\b\x.png`},
		{`c:/users//b/./x.png`, PathWindows, `c:\users\b\x.png`},
		{`C:\..\x.png`, PathWindows, `c:\x.png`},
		{`\\Server\Share\Out\..\x.png`, PathWindows, `\\server\share\x.png`},
		{`/Users/A/../B/x.PNG`, PathMacOS, `/users/b/x.png`},
		{`/home/A//B/./x.PNG`, PathLinux, `/home/A/B/x.PNG`},
	} {
		got, err := CleanPath(c.in, c.style)
		if err != nil || got != c.want {
			t.Errorf("CleanPath(%q, %s) = %q, %v; want %q", c.in, c.style, got, err, c.want)
		}
	}
	if _, err := CleanPath("/x.png", ""); err == nil {
		t.Error("no path style: no error")
	}
	if _, err := Recipe(with(func(r *Request) { r.OutPath = "/x.png" })); err == nil {
		t.Error("an out_path with no path style: no error")
	}
}
