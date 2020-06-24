/*************************************************************************
 * Copyright 2017 John Floren. All rights reserved.
 * Contact: <john@jfloren.net>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
)

var (
	zkRoot string
	state  zkState
)

type zkState struct {
	CurrentNote int
	NextNoteId  int
	Notes       map[int]NoteMeta
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
	body string
}

func main() {
	flag.Parse()

	// All commands take the form "zk <command> <command args>"
	// Specifying simply "zk" should show the current note's summary & subnotes
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
		showNote(args)
	case "new", "n":
		newNote(args)
	case "up", "u":
		if state.CurrentNote != 0 {
			p := state.Notes[state.CurrentNote].Parent
			changeLevel(p)
			showNote([]string{})
		}
	case "edit", "e":
		editNote(args)
	case "print", "p":
		printNote(args)
	case "tree", "t":
		printTree(args)
	case "link":
		linkNote(args)
	case "unlink":
		unlinkNote(args)
	case "addfile":
		addFile(args)
	case "listfiles", "ls":
		listFiles(args)
	default:
		if flag.NArg() == 1 {
			id, err := strconv.Atoi(flag.Arg(0))
			if err != nil {
				log.Fatalf("couldn't parse id: %v", err)
			}
			// we've been given an argument, try to change to the specified note
			changeLevel(id)
			showNote([]string{})
		} else {
			showNote([]string{})
		}
	}

	writeState()
}

func initDeck() {
	state.CurrentNote = 0
	state.NextNoteId = 1
	state.Notes = make(map[int]NoteMeta)
	makeNote(0, 0, "Top Level\n")
	writeState()
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
func makeNote(id int, parent int, body string) error {
	// First verify that the id doesn't already exist
	if m, ok := state.Notes[id]; ok {
		return fmt.Errorf("a note with id %v already exists: %v", id, m)
	}
	meta := NoteMeta{Id: id}
	s := bufio.NewScanner(bytes.NewBuffer([]byte(body)))
	if s.Scan() {
		meta.Title = s.Text()
	}
	// TODO: the parent might not always be the current note, but for now it is
	//meta.Parent = state.CurrentNote
	meta.Parent = parent

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
		log.Fatalf("usage: zk new [note]")
	}
	// read in a body
	fmt.Println("Enter note; the first line will be the title. Ctrl-D when done.")
	body, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal("couldn't read body text: %v", err)
	}

	err = makeNote(state.NextNoteId, targetNote, string(body))
	if err != nil {
		log.Fatal("couldn't create note with id %v: %v", state.NextNoteId, err)
	}
	state.NextNoteId++
}

func showNote(args []string) {
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

	fmt.Printf("%d %s\n", note.Id, note.Title)
	for _, sn := range subnotes {
		fmt.Printf("	%d %s\n", sn.Id, sn.Title)
	}
}

func changeLevel(id int) {
	if _, ok := state.Notes[id]; !ok {
		log.Fatalf("invalid note id %v", id)
	}

	state.CurrentNote = id
}

func addFile(args []string) {
	var err error
	var srcPath string
	target := state.CurrentNote
	switch len(args) {
	case 1:
		// just a filename, append to current note
		srcPath = args[0]
	case 2:
		// note number followed by filename
		target, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("can't parse id: %v")
		}
		srcPath = args[1]
	}
	// Verify that the source file exists
	_, err = os.Stat(srcPath)
	if err != nil {
		log.Fatalf("Cannot find source file %v: %v", err)
	}
	src, err := os.Open(srcPath)
	if err != nil {
		log.Fatalf("Cannot open source file %v: %v", srcPath, err)
	}

	// Verify that the destination files directory exists
	p := filepath.Join(zkRoot, fmt.Sprintf("%d", target), "files")
	_, err = os.Stat(p)
	if err != nil {
		log.Fatalf("Cannot open %v: %v", p, err)
	}

	// Copy the file into the directory
	base := filepath.Base(srcPath)
	if base == "." {
		log.Fatalf("Cannot find base name for %v")
	}
	dstPath := filepath.Join(p, base)
	dst, err := os.Create(dstPath)
	if err != nil {
		log.Fatalf("Cannot create destination file %v: %v", dstPath, err)
	}

	_, err = io.Copy(dst, src)
	if err != nil {
		log.Printf("Problem copying %v to %v: %v", srcPath, dstPath, err)
	}

	// Re-read the note to update the metadata
	n, err := readNote(target)
	if err != nil {
		log.Fatalf("Failed to read note %v: %v", target, err)
	}
	fmt.Printf("Files for [%d] %v:\n", n.Id, n.Title)
	for _, f := range n.Files {
		fmt.Printf("	%v\n", f)
	}
}

func listFiles(args []string) {
	var err error
	target := state.CurrentNote
	if len(args) == 1 {
		target, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("can't parse id: %v")
		}
	}
	// Re-read the note to update the metadata
	n, err := readNote(target)
	if err != nil {
		log.Fatalf("Failed to read note %v: %v", target, err)
	}
	fmt.Printf("Files for [%d] %v:\n", n.Id, n.Title)
	for _, f := range n.Files {
		fmt.Printf("	%v\n", f)
	}
}

func editNote(args []string) {
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

// Arg 0: source
// Arg 1: target
func linkNote(args []string) {
	var src, dst int
	var err error
	if len(args) == 2 {
		src, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("can't parse id %v: %v", args[0], err)
		}
		dst, err = strconv.Atoi(args[1])
		if err != nil {
			log.Fatalf("can't parse id %v: %v", args[1], err)
		}
	} else {
		log.Fatalf("must specify source (note to be linked) and destination (note into which it will be linked)")
	}
	if note, ok := state.Notes[dst]; ok {
		note.Subnotes = append(note.Subnotes, src)
		state.Notes[dst] = note
		err = writeNoteMetadata(state.Notes[dst])
		if err != nil {
			log.Fatalf("Failed to write back metadata: %v", err)
		}
	}
}

// Unlink the specified note from the current note
func unlinkNote(args []string) {
	var err error
	target := state.CurrentNote
	var child int
	if len(args) == 1 {
		child, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("can't parse id: %v", err)
		}
	} else {
		log.Fatal("must specify a note id to unlink")
	}
	if note, ok := state.Notes[target]; ok {
		children := state.Notes[target].Subnotes
		var newChildren []int
		for _, sn := range children {
			if sn != child {
				newChildren = append(newChildren, sn)
			}
		}
		note.Subnotes = newChildren
		state.Notes[target] = note
		err = writeNoteMetadata(state.Notes[target])
		if err != nil {
			log.Fatalf("Failed to write back metadata: %v", err)
		}
	} else {
		log.Fatalf("couldn't find note %v", target)
	}
}

func printTree(args []string) {
	var err error
	target := 0
	if len(args) == 1 {
		target, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("couldn't parse note id %v: %v", args[0], err)
		}
	}
	printTreeRecursive(0, target)
}

func printTreeRecursive(depth, id int) {
	if note, ok := state.Notes[id]; ok {
		for i := 0; i < depth; i++ {
			fmt.Printf("	")
		}
		fmt.Printf("%d %s\n", note.Id, note.Title)
		for _, sn := range note.Subnotes {
			printTreeRecursive(depth+1, sn)
		}
	}
}
