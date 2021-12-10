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
	if _, err = z.NewNote(0, "Testing\n"); err != nil {
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
	if _, err = z.NewNote(0, "Testing\n"); err != nil {
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
	if _, err = z.NewNote(0, "Testing\n"); err != nil {
		t.Fatal(err)
	}

	// And then make a child of that note
	if _, err = z.NewNote(1, "Child note\n"); err != nil {
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

	// Create a new note. Its ID will be 1.
	if _, err = z.NewNote(0, "Testing\n"); err != nil {
		t.Fatal(err)
	}

	// And then make a child of that note
	if _, err = z.NewNote(1, "Child note\n"); err != nil {
		t.Fatal(err)
	}

	// Now link the new note (id will be 2) to note 0
	// Tree now looks like this:
	// 0
	// 	1
	// 		2
	// 	2
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

	// Now we'll also unlink note 2 from note 1, orphaning it.
	if err = z.UnlinkNote(1, 2); err != nil {
		t.Fatal(err)
	}
	// Make sure it comes back in the list of orphans
	if orphans := z.GetOrphans(); len(orphans) != 1 {
		t.Fatalf("Got wrong number of orphans, expected 1 got %v (list: %v)", len(orphans), orphans)
	} else if orphans[0].Id != 2 {
		t.Fatalf("Got the wrong orphan back: %v", orphans[0])
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

func TestFiles(t *testing.T) {
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
	if _, err = z.NewNote(0, "Testing\n"); err != nil {
		t.Fatal(err)
	}

	// Make a temp file
	f, err := ioutil.TempFile("", "foo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	if err = z.AddFile(1, f.Name(), "foo"); err != nil {
		t.Fatal(err)
	}

	// Now make sure it actually got added
	var md NoteMeta
	if md, err = z.GetNoteMeta(1); err != nil {
		t.Fatal(err)
	}
	if len(md.Files) != 1 && md.Files[0] != "foo" {
		t.Fatalf("Got bad files list: %v", md.Files)
	}

	// Check that we can get a path to it
	if _, err := z.GetFilePath(1, "foo"); err != nil {
		t.Fatalf("Couldn't get path to file: %v", err)
	}

	// And remove it
	if err = z.RemoveFile(1, "foo"); err != nil {
		t.Fatal(err)
	}

	// Now make sure it actually got removed
	if md, err = z.GetNoteMeta(1); err != nil {
		t.Fatal(err)
	}
	for _, f := range md.Files {
		if f == "foo" {
			t.Fatal("File still exists!")
		}
	}
}

func TestGrep(t *testing.T) {
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
	body := `Test note title xyzzy
This is the note. Not every line contains a match.
There are three lines which will match a regex that consists of an x, followed by some non-space chars, followed by a y, and this is not one.
But this line matches: x12y
And xFFFF*y matches too, as does the title.`
	if _, err = z.NewNote(0, body); err != nil {
		t.Fatal(err)
	}

	// Now do a grep
	var r chan *GrepResult
	if r, err = z.Grep(`x\S+y`, []int{}); err != nil {
		t.Fatal(err)
	}
	var count int
	for _ = range r {
		count++
	}
	if count != 3 {
		t.Fatalf("Got bad results, expected 3 got %v\n", count)
	}
}
