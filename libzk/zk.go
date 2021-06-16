package zk

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type zkState struct {
	NextNoteId int
	Notes      map[int]NoteMeta
}

type NoteMeta struct {
	Id       int
	Title    string
	Subnotes []int
	Files    []string
	Parent   int
}

type Note struct {
	NoteMeta
	Body string
}

type ZK struct {
	root  string
	state zkState
}

// InitZK will initialize a new zk with the specified path as the
// root directory. If the path already exists, it must be empty.
func InitZK(root string) error {
	z := &ZK{
		root: root,
		state: zkState{
			NextNoteId: 1,
			Notes:      make(map[int]NoteMeta),
		},
	}

	// There should be nothing in the directory, if it exists
	if contents, err := ioutil.ReadDir(z.root); err == nil {
		if len(contents) > 0 {
			return errors.New("specified root already contains files/directories")
		}
	}

	// Creating the directory is fine
	os.MkdirAll(z.root, 0755)

	// Generate a top-level note
	if err := z.makeNote(0, 0, "Top Level\n"); err != nil {
		return err
	}
	return z.writeState()
}

// NewZK creates a ZK object rooted at the specified directory.
// The directory should have been previously initialized with the InitZK function.
func NewZK(root string) (z *ZK, err error) {
	z = &ZK{
		root: root,
	}

	// Attempt to read a state file.
	err = z.readState()

	return
}

func (z *ZK) Close() {
	z.writeState()
}

// GetNote returns the full contents of the specified note ID,
// including the body. Unlike GetNoteMeta, it actually reads
// from the disk and will update the in-memory state if out of sync.
func (z *ZK) GetNote(id int) (note Note, err error) {
	return z.readNote(id)
}

// Read a note by id from the filesystem, updating our metadata map
// as we do it.
func (z *ZK) readNote(id int) (result Note, err error) {
	// read the metadata
	result.NoteMeta, err = z.readNoteMetadata(id)
	if err != nil {
		return
	}

	p := filepath.Join(z.root, fmt.Sprintf("%d", id))
	// list the files -- we want to double check in case somebody did something stupid manually
	result.Files = []string{}
	files, err := ioutil.ReadDir(filepath.Join(p, "files"))
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		result.Files = append(result.Files, f.Name())
	}

	// read the body
	b, err := ioutil.ReadFile(filepath.Join(p, "body"))
	if err != nil {
		return result, err
	}
	result.Body = string(b)

	// get the title
	s := bufio.NewScanner(bytes.NewBuffer(b))
	if s.Scan() {
		result.Title = s.Text()
	}

	// now write back the metadata to our map
	z.state.Notes[id] = result.NoteMeta
	if err := z.writeNoteMetadata(result.NoteMeta); err != nil {
		return result, err
	}

	return result, nil

}

func (z *ZK) GetNoteMeta(id int) (md NoteMeta, err error) {
	var ok bool
	if md, ok = z.state.Notes[id]; !ok {
		err = fmt.Errorf("Note %d not found", id)
	}
	return
}

func (z *ZK) NewNote(parent int, body string) (int, error) {
	id := z.state.NextNoteId
	err := z.makeNote(id, parent, body)
	if err != nil {
		return 0, err
	}
	z.state.NextNoteId++
	return id, z.writeState()
}

// makeNote does NOT write the state file
func (z *ZK) makeNote(id, parent int, body string) error {
	// First verify that the id doesn't already exist
	if m, ok := z.state.Notes[id]; ok {
		return fmt.Errorf("a note with id %v already exists: %v", id, m)
	}
	meta := NoteMeta{Id: id}
	s := bufio.NewScanner(bytes.NewBuffer([]byte(body)))
	if s.Scan() {
		meta.Title = s.Text()
	}

	meta.Parent = parent

	// make the note dir
	path := filepath.Join(z.root, fmt.Sprintf("%d", id))
	err := os.MkdirAll(path, 0700)
	if err != nil {
		return err
	}

	// Now create the subdirectories and files that go in it
	err = ioutil.WriteFile(filepath.Join(path, "body"), []byte(body), 0700)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Join(path, "files"), 0700)
	if err != nil {
		return err
	}

	err = z.writeNoteMetadata(meta)
	if err != nil {
		return err
	}

	// At this point we should be ok to append ourselves to the parent's subnote list
	// very dumb check to make sure we're not note 0
	if id != 0 {
		pmeta := z.state.Notes[meta.Parent]
		pmeta.Subnotes = append(pmeta.Subnotes, id)
		z.state.Notes[meta.Parent] = pmeta

		// Now write it out to the file
		err = z.writeNoteMetadata(z.state.Notes[meta.Parent])
		if err != nil {
			return err
		}
	}

	// We've made all the files, write the metadata into the map.
	z.state.Notes[id] = meta

	return nil
}

func (z *ZK) UpdateNote(id int, body string) error {
	// Make sure the note exists
	var meta NoteMeta
	var ok bool
	if meta, ok = z.state.Notes[id]; !ok {
		return fmt.Errorf("Note %d not found", id)
	}

	// Figure out the new title & update metadata
	s := bufio.NewScanner(bytes.NewBuffer([]byte(body)))
	if s.Scan() {
		meta.Title = s.Text()
	}

	// Write out to the body file
	path := filepath.Join(z.root, fmt.Sprintf("%d", id))
	if err := ioutil.WriteFile(filepath.Join(path, "body"), []byte(body), 0700); err != nil {
		return err
	}

	// Finally, update metadata
	z.state.Notes[id] = meta
	if err := z.writeNoteMetadata(meta); err != nil {
		return err
	}

	return nil
}

// MetadataDump returns the entire contents of the in-memory state.
// This can be useful when walking the entire tree.
func (z *ZK) MetadataDump() map[int]NoteMeta {
	return z.state.Notes
}

// GetNoteBodyPath returns an absolute path to the given note's body
// file, suitable for passing to an editor. Note that changing the
// note's title here by editing this file will not change the title
// in the in-memory metadata until GetNote, Rescan, or another function
// which reads and parses the on-disk files is called.
func (z *ZK) GetNoteBodyPath(id int) (path string, err error) {
	if _, ok := z.state.Notes[id]; !ok {
		err = fmt.Errorf("Note %d not found", id)
		return
	}
	path = filepath.Join(z.root, fmt.Sprintf("%d", id), "body")
	return
}

// LinkNote links the specified note as a child of the parent note.
func (z *ZK) LinkNote(parent, id int) error {
	// Get the parent
	p, ok := z.state.Notes[parent]
	if !ok {
		return fmt.Errorf("Parent note %d not found", parent)
	}

	// Make sure the child exists
	_, ok = z.state.Notes[id]
	if !ok {
		return fmt.Errorf("Note %d not found", id)
	}

	// Add the link
	for i := range p.Subnotes {
		if p.Subnotes[i] == id {
			// it's already linked
			return nil
		}
	}
	p.Subnotes = append(p.Subnotes, id)

	// Write state & metadata file
	z.state.Notes[parent] = p
	return z.writeNoteMetadata(p)
}

// UnlinkNote removes the specified note from the parent note's subnotes
func (z *ZK) UnlinkNote(parent, id int) error {
	// Get the child
	child, ok := z.state.Notes[id]
	if !ok {
		return fmt.Errorf("Child note %d not found", id)
	}
	// Get the parent
	p, ok := z.state.Notes[parent]
	if !ok {
		return fmt.Errorf("Parent note %d not found", parent)
	}

	// Remove the link
	var newSubnotes []int
	for _, sn := range p.Subnotes {
		if sn != id {
			newSubnotes = append(newSubnotes, sn)
		}
	}
	p.Subnotes = newSubnotes

	// If we've just removed the "parent" note, re-parent it to note 0.
	// We could look for other notes with this as a subnote, but
	// that seems more confusing to users.
	if parent == child.Parent {
		child.Parent = 0
		z.state.Notes[id] = child
	}

	// Write state & metadata file
	z.state.Notes[parent] = p
	return z.writeNoteMetadata(p)
}

// AddFile copies the file at the specified path into the given note's files.
// If dstName is not empty, the resulting file will be given that name.
func (z *ZK) AddFile(id int, path string, dstName string) error {
	// Make sure that note actually exists
	dstNote, ok := z.state.Notes[id]
	if !ok {
		return fmt.Errorf("Note %d not found", id)
	}
	// Verify that the source file exists
	var err error
	_, err = os.Stat(path)
	if err != nil {
		return fmt.Errorf("Cannot find source file %v: %v", dstName, err)
	}
	src, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("Cannot open source file %v: %v", path, err)
	}

	// Verify that the destination files directory exists
	p := filepath.Join(z.root, fmt.Sprintf("%d", id), "files")
	_, err = os.Stat(p)
	if err != nil {
		return fmt.Errorf("Cannot open %v: %v", p, err)
	}

	// Copy the file into the directory
	base := dstName
	if base == "" {
		base = filepath.Base(path)
		if base == "." {
			return fmt.Errorf("Cannot find base name for %v", path)
		}
	}
	// Make sure there's not already a file with that name in the destination
	for _, f := range dstNote.Files {
		if f == base {
			return fmt.Errorf("File named %v already exists for note %d", base, id)
		}
	}

	dstPath := filepath.Join(p, base)
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("Cannot create destination file %v: %v", dstPath, err)
	}

	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("Problem copying %v to %v: %v", path, dstPath, err)
	}

	// Re-read the note to update the metadata
	_, err = z.readNote(id)
	if err != nil {
		return fmt.Errorf("Failed to read note %v: %v", id, err)
	}
	return nil
}

// RemoveFile removes the specified file from the note.
func (z *ZK) RemoveFile(id int, name string) error {
	// Make sure that note actually exists
	dstNote, ok := z.state.Notes[id]
	if !ok {
		return fmt.Errorf("Note %d not found", id)
	}

	// First remove the file from the disk
	p := filepath.Join(z.root, fmt.Sprintf("%d", id), "files", name)
	if err := os.Remove(p); err != nil {
		return err
	}

	// Now take it out of the metadata
	var newFiles []string
	for _, f := range dstNote.Files {
		if f != name {
			newFiles = append(newFiles, f)
		}
	}
	dstNote.Files = newFiles

	// And update metadata
	z.state.Notes[id] = dstNote
	return z.writeNoteMetadata(dstNote)
}

// GetFilePath returns an absolute path to a given file within a note
func (z *ZK) GetFilePath(id int, name string) (string, error) {
	return z.getFilePath(id, name)
}

func (z *ZK) getFilePath(id int, name string) (string, error) {
	// Make sure that note actually exists
	_, ok := z.state.Notes[id]
	if !ok {
		return "", fmt.Errorf("Note %d not found", id)
	}
	p := filepath.Join(z.root, fmt.Sprintf("%d", id), "files", name)
	if _, err := os.Stat(p); err != nil {
		return "", err
	}
	return p, nil
}

// GetFileReader returns an io.Reader attached to the specified file within a note
func (z *ZK) GetFileReader(id int, name string) (io.Reader, error) {
	p, err := z.getFilePath(id, name)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

// Rescan will attempt to re-derive the state from the contents of the zk
// directory. Useful if you have manually messed with the directories, or
// if things just seem out of sync.
func (z *ZK) Rescan() error {
	state, err := z.deriveState()
	if err != nil {
		// give up
		return err
	}
	z.state = state
	return nil
}

// GrepResult contains a single matching line returned from the Grep function.
// The Note field is the id of the note which matched
// The Line field is the text of the note which matched.
type GrepResult struct {
	Note  NoteMeta
	Line  string
	Error error
}

type oneGrep struct {
	c chan *GrepResult
}

func (z *ZK) grep(n NoteMeta, pattern *regexp.Regexp, c chan *oneGrep) {
	// Create a channel of GrepResults and hand it back up to the master routine
	res := make(chan *GrepResult)
	defer close(res)
	c <- &oneGrep{res}

	// Get a reader on the note body
	p := filepath.Join(z.root, fmt.Sprintf("%d", n.Id), "body")
	f, err := os.Open(p)
	if err != nil {
		res <- &GrepResult{Note: n, Error: err}
		return
	}
	defer f.Close()

	// Now walk it, looking for any matching lines
	rdr := bufio.NewReader(f)
	for {
		if s, err := rdr.ReadString('\n'); err != nil && err != io.EOF {
			// Legit error, pass it up
			res <- &GrepResult{Note: n, Error: err}
		} else {
			if pattern.MatchString(s) {
				// match!
				res <- &GrepResult{Note: n, Line: strings.TrimSuffix(s, "\n")}
			}
			if err == io.EOF {
				break
			}
		}
	}
}

// TreeGrep searches note bodies for a regular expression. It takes as arguments
// a regular expression string and a note ID. That note, and the entire tree of
// subnotes below it, are searched.
func (z *ZK) TreeGrep(pattern string, root int) (c chan *GrepResult, err error) {
	// Make sure the specified root actually exists
	if _, ok := z.state.Notes[root]; !ok {
		err = fmt.Errorf("Note %d does not exist", root)
		return
	}
	// Simple lambda function to walk the tree and build up a list of notes to search
	var f func(int) []int
	f = func(id int) []int {
		note, ok := z.state.Notes[id]
		if !ok {
			// this shouldn't happen
			return []int{}
		}
		l := []int{note.Id}
		for _, n := range note.Subnotes {
			l = append(l, f(n)...)
		}
		return l
	}
	return z.Grep(pattern, f(root))
}

// Grep searches note bodies for a regular expression and returns a channel of *GrepResult.
// If the notes parameter is non-empty, it will restrict the search to only the specified note IDs.
func (z *ZK) Grep(pattern string, notes []int) (c chan *GrepResult, err error) {
	c = make(chan *GrepResult, 1024)
	results := make(chan *oneGrep)

	// Check the regular expression
	var re *regexp.Regexp
	if re, err = regexp.Compile(pattern); err != nil {
		return
	}

	// Figure out which notes we're working with. If none were passed, use all of them.
	if len(notes) == 0 {
		for _, v := range z.state.Notes {
			notes = append(notes, v.Id)
		}
	}
	var toSearch []NoteMeta
	for _, n := range notes {
		if md, ok := z.state.Notes[n]; ok {
			toSearch = append(toSearch, md)
		}
	}

	// Fire off a goroutine for each note
	for _, n := range toSearch {
		go z.grep(n, re, results)
	}

	// Now fire the goroutine which relays from those notes to the reader.
	// We do it like this so we get all results from one note at a time.
	go func() {
		nGrep := len(toSearch)

		for g := range results {
			for r := range g.c {
				c <- r
			}
			nGrep--
			if nGrep == 0 {
				break
			}
		}

		close(c)
	}()

	return c, nil
}

// GetOrphans returns a list of "orphaned" notes, notes which are not the subnote of
// any other note.
func (z *ZK) GetOrphans() (orphans []NoteMeta) {
	// For every note...
orphanLoop:
	for id, meta := range z.state.Notes {
		// Note 0 can never be an orphan.
		if id == 0 {
			continue orphanLoop
		}
		// walk the set of notes *again* to see if it's a sub-note of any of them.
		for _, candidate := range z.state.Notes {
			for i := range candidate.Subnotes {
				if candidate.Subnotes[i] == id {
					// We found the ID in a list of subnotes, on to the next potential orphan!
					continue orphanLoop
				}
			}
		}
		// If we got this far, the note was not a subnote of *any* other note.
		orphans = append(orphans, meta)
	}
	return
}
