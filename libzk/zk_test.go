package zk

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestNewZK(t *testing.T) {
	dir, err := ioutil.TempDir("", "zk")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err := InitZK(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := NewZK(dir); err != nil {
		t.Fatal(err)
	}
}

func TestNewNote(t *testing.T) {
	var err error
	dir, err := ioutil.TempDir("", "zk")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err = InitZK(dir); err != nil {
		t.Fatal(err)
	}
	var z *ZK
	if z, err = NewZK(dir); err != nil {
		t.Fatal(err)
	}

	// Create a note
	if err = z.NewNote(0, "Testing\n"); err != nil {
		t.Fatal(err)
	}

	// Now make sure it is listed as a child of note 0
	var md NoteMeta
	if md, err = z.GetNoteMeta(0); err != nil {
		t.Fatal(err)
	}
	if len(md.Subnotes) != 1 {
		t.Fatalf("Wrong number of subnotes on note 0, got %d should be 1", len(md.Subnotes))
	}

	// And make sure the new note (guaranteed to be id 1) is ok
	if md, err = z.GetNoteMeta(1); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateNote(t *testing.T) {
	var err error
	dir, err := ioutil.TempDir("", "zk")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err = InitZK(dir); err != nil {
		t.Fatal(err)
	}
	var z *ZK
	if z, err = NewZK(dir); err != nil {
		t.Fatal(err)
	}

	// Create a note
	if err = z.NewNote(0, "Testing\n"); err != nil {
		t.Fatal(err)
	}

	// Now update it, check title
	newBody := string("Foo\nLong live the new flesh")
	if err = z.UpdateNote(1, newBody); err != nil {
		t.Fatal(err)
	}
	var md Note
	if md, err = z.GetNote(1); err != nil {
		t.Fatal(err)
	}
	if md.Title != "Foo" {
		t.Fatalf("Invalid title on note: %v", md.Title)
	}
	if md.Body != newBody {
		t.Fatalf("Invalid note body: %v", md.Body)
	}
}