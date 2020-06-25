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

func (z *ZK) NewNote(parent int, body string) error {
	err := z.makeNote(z.state.NextNoteId, parent, body)
	if err != nil {
		return err
	}
	z.state.NextNoteId++
	return z.writeState()
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