package gobundle

import (
	"os"
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
