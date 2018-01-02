package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"path/filepath"
	"io/ioutil"
	"io"
	"sort"
	"os/user"
	"os/exec"
)

var (
	zkRoot string
	state	zkState
)

type zkState struct {
	CurrentNote int
	NextNoteId	int
	Notes	map[int]NoteMeta
}

type NoteMeta struct {
	Id	int
	Title	string
	Subnotes []int
	Files []string
	Parent	int
}

type Note struct {
	NoteMeta
	body	string
}

func main() {
	flag.Parse()

	// All commands take the form "z <command> <command args>"
	// Specifying simply "z" should show the current note's summary & subnotes
	var cmd string
	if len(flag.Args()) > 0 {
		cmd = flag.Arg(0)
	}
	var args []string
	if len(flag.Args()) > 1 {
		args = flag.Args()[1:]
	}

	// We default to keeping notes in ~/zk, we'll add multiple decks later maybe
	usr, err := user.Current()
	if err != nil {
		log.Fatalf("couldn't get user info: %v", err)
	}
	zkRoot = filepath.Join(usr.HomeDir, "zk")
	os.MkdirAll(zkRoot, 0755)

	if cmd != "init" {
		readState()
	}

	switch cmd {
	case "init":
		initDeck()
	case "show", "s":
		show(args)
	case "new", "n":
		newNote(args)
	case "up", "u":
		if state.CurrentNote != 0 {
			p := state.Notes[state.CurrentNote].Parent
			changeLevel(p)
			show([]string{})
		}
	case "edit", "e":
		edit(args)
	case "print", "p":
		printNote(args)
	default:
		if flag.NArg() == 1 {
			id, err := strconv.Atoi(flag.Arg(0))
			if err != nil {
				log.Fatalf("couldn't parse id: %v", err)
			}
			// we've been given an argument, try to change to the specified note
			changeLevel(id)
			show([]string{})
		} else {
			show([]string{})
		}
	}

	writeState()
}

func initDeck() {
	state.CurrentNote = 0
	state.NextNoteId = 1
	state.Notes = make(map[int]NoteMeta)
	makeNote(0, "Top Level\n")
	writeState()
}

func readState() {
	p := filepath.Join(zkRoot, "state")
	fd, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()

	dec := json.NewDecoder(fd)
	err = dec.Decode(&state)
	if err != nil && err != io.EOF {
		log.Fatalf("failure parsing state file: %v", err)
	}
}

func writeState() {
	p := filepath.Join(zkRoot, "state")
	fd, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()

	fd.Truncate(0)
	fd.Seek(0, 0)
	enc := json.NewEncoder(fd)
	enc.Encode(state)
	fd.Sync()
}

func readNoteMetadata(id int) (NoteMeta, error) {
	var meta NoteMeta
	p := filepath.Join(zkRoot, fmt.Sprintf("%d", id), "metadata")
	fd, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return meta, err
	}
	defer fd.Close()

	dec := json.NewDecoder(fd)
	err = dec.Decode(&meta)
	if err != nil && err != io.EOF {
		return meta, fmt.Errorf("failure parsing state file: %v", err)
	}
	return meta, nil
}

// infers where to write based on the metadata
func writeNoteMetadata(meta NoteMeta) error {
	p := filepath.Join(zkRoot, fmt.Sprintf("%d", meta.Id), "metadata")
	fd, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer fd.Close()

	fd.Truncate(0)
	fd.Seek(0, 0)
	enc := json.NewEncoder(fd)
	enc.Encode(meta)
	fd.Sync()
	return nil
}

// Read a note by id from the filesystem, updating our metadata map
// as we do it.
func readNote(id int) (*Note, error) {
	var err error
	result := &Note{}

	// read the metadata
	result.NoteMeta, err = readNoteMetadata(id)
	if err != nil {
		return nil, err
	}

	p := filepath.Join(zkRoot, fmt.Sprintf("%d", id))

	// list the files -- we want to double check in case somebody did something stupid manually
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
		return nil, err
	}
	result.body = string(b)

	// get the title
	s := bufio.NewScanner(bytes.NewBuffer(b))
	if s.Scan() {
		result.Title = s.Text()
	}

	// now write back the metadata to our map
	state.Notes[id] = result.NoteMeta

	return result, nil
}


// This is called to set up the files for a new note, once the id has been determined.
func makeNote(id int, body string) error {
	// First verify that the id doesn't already exist
	if m, ok := state.Notes[id]; ok {
		return fmt.Errorf("a note with id %v already exists: %v", id, m)
	}
	meta := NoteMeta{ Id: id }
	s := bufio.NewScanner(bytes.NewBuffer([]byte(body)))
	if s.Scan() {
		meta.Title = s.Text()
	}
	// TODO: the parent might not always be the current note, but for now it is
	meta.Parent = state.CurrentNote

	// make the note dir
	path := filepath.Join(zkRoot, fmt.Sprintf("%d", id))
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

	err = writeNoteMetadata(meta)
	if err != nil {
		return err
	}

	// At this point we should be ok to append ourselves to the parent's subnote list
	// very dumb check to make sure we're not note 0 (bootstrapping
	if id != 0 {
		pmeta := state.Notes[meta.Parent]
		pmeta.Subnotes = append(pmeta.Subnotes, id)
		state.Notes[meta.Parent] = pmeta

		// Now write it out to the file
		err = writeNoteMetadata(state.Notes[meta.Parent])
		if err != nil {
			return err
		}
	}

	// We've made all the files, write the metadata into the map.
	state.Notes[id] = meta

	return nil
}

func newNote(args []string) {
	// read in a body
	fmt.Println("Enter note; the first line will be the title. Ctrl-D when done.")
	body, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal("couldn't read body text: %v", err)
	}

	err = makeNote(state.NextNoteId, string(body))
	if err != nil {
		log.Fatal("couldn't create note with id %v: %v", state.NextNoteId, err)
	}
	state.NextNoteId++
}

func show(args []string) {
	var targetNote int
	var err error

	if len(args) == 0 {
		targetNote = state.CurrentNote
	} else if len(args) == 1 {
		targetNote, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("failed to parse specified note %v: %v", args[0], err)
		}
	} else {
		log.Fatalf("usage: zk show [note]")
	}

	note, err := readNote(targetNote)
	if err != nil {
		log.Fatalf("couldn't read note: %v", err)
	}

	var subnotes []*Note
	for _, n := range note.Subnotes {
		sn, err := readNote(n)
		if err != nil {
			log.Fatalf("failed to read subnote %v: %v", n, err)
		}
		subnotes = append(subnotes, sn)
	}

	// Sort the subnotes by ID
	sort.Slice(subnotes, func(i, j int) bool { return subnotes[i].Id < subnotes[j].Id })

	fmt.Printf("id:%d %s\n\n", note.Id, note.Title)
	for _, sn := range subnotes {
		fmt.Printf("[id:%d]    %s\n",sn.Id, sn.Title)
	}
}

func changeLevel(id int) {
	if _, ok := state.Notes[id]; !ok {
		log.Fatalf("invalid note id %v", id)
	}

	state.CurrentNote = id
}

func edit(args []string) {
	var err error
	target := state.CurrentNote
	if len(args) == 1 {
		target, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("can't parse id: %v")
		}
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	p := filepath.Join(zkRoot, fmt.Sprintf("%d", target), "body")
	cmd := exec.Command(editor, p)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	cmd.Wait()
}

func printNote(args []string) {
	target := filepath.Join(zkRoot, fmt.Sprintf("%d", state.CurrentNote), "body")
	if len(args) == 1 {
		target = filepath.Join(zkRoot, args[0], "body")
	}
	if f, err := os.Open(target); err == nil {
		defer f.Close()
		io.Copy(os.Stdout, f)
	} else {
		log.Fatalf("couldn't read note: %v", err)
	}
}