package gobundle

import (
	"os"
	"strings"
	"testing"
)

func TestBundle(t *testing.T) {
	os.Chdir("./testdata/example")
	out, err := Bundle("./main.go")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(out)
}

func TestBundle_child(t *testing.T) {
	t.Skip()
	os.Chdir("./testdata/example")
	out, err := Bundle("./child")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(out)
}

func TestBundle_parent(t *testing.T) {
	t.Skip()
	os.Chdir("./testdata/example")
	out, err := Bundle("./parent")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(out)
}

func TestBundle_embedding(t *testing.T) {
	os.Chdir("./testdata/example")
	out, err := Bundle("./embedding")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(out)
}

func TestBundle_unfolded_const_comment(t *testing.T) {
	os.Chdir("./testdata/unfolded-const-comment")
	out, err := Bundle(".")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(out)

	// I want fix this test case. But this bug is caused by go/printer package.
	shouldContain := []string{
		`
const (
	unfold0__Dummy1 = "" // comment

	unfold0__Dummy2// comment
	= ""
)`,
	}
	for _, str := range shouldContain {
		if !strings.Contains(out, str) {
			t.Errorf("output should contain: %s", str)
		}
	}
}
