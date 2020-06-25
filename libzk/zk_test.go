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

func TestLinkNote(t *testing.T) {
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

	// And then make a child of that note
	if err = z.NewNote(1, "Child note\n"); err != nil {
		t.Fatal(err)
	}

	// Now link the new note (id will be 2) to note 0
	if err = z.LinkNote(0, 2); err != nil {
		t.Fatal(err)
	}

	// Verify that it's listed as a child of both 0 and 1
	var md NoteMeta
	if md, err = z.GetNoteMeta(0); err != nil {
		t.Fatal(err)
	}
	if !containsSubnote(md, 2) {
		t.Fatalf("Note 0 doesn't contain new note as subnote")
	}
	if md, err = z.GetNoteMeta(1); err != nil {
		t.Fatal(err)
	}
	if !containsSubnote(md, 2) {
		t.Fatalf("Note 1 doesn't contain new note as subnote")
	}

}

func TestUnlinkNote(t *testing.T) {
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

	// And then make a child of that note
	if err = z.NewNote(1, "Child note\n"); err != nil {
		t.Fatal(err)
	}

	// Now link the new note (id will be 2) to note 0
	if err = z.LinkNote(0, 2); err != nil {
		t.Fatal(err)
	}

	// Verify that it's listed as a child of 0
	var md NoteMeta
	if md, err = z.GetNoteMeta(0); err != nil {
		t.Fatal(err)
	}
	if !containsSubnote(md, 2) {
		t.Fatalf("Note 0 doesn't contain new note as subnote")
	}

	// Unlink it from 0...
	if err = z.UnlinkNote(0, 2); err != nil {
		t.Fatal(err)
	}
	if md, err = z.GetNoteMeta(0); err != nil {
		t.Fatal(err)
	}
	if containsSubnote(md, 2) {
		t.Fatalf("Note 0 still improperly contains note 2: %+v", md)
	}
}

func containsSubnote(md NoteMeta, id int) bool {
	for _, c := range md.Subnotes {
		if c == id {
			return true
		}
	}
	return false
}