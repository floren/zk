/*************************************************************************
 * Copyright 2017 John Floren. All rights reserved.
 * Contact: <john@jfloren.net>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	zk "github.com/floren/zk/libzk"
)

var (
	configFile = flag.String("config", "", "Path to alternate config file")

	cfg Config
	z   *zk.ZK
)

type Config struct {
	ZKRoot        string
	CurrentNoteId int
}

func defaultConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	pth := filepath.Join(configDir, "zk")
	if err = os.MkdirAll(pth, 0755); err != nil {
		return "", err
	}
	pth = filepath.Join(pth, "zkconfig")
	return pth, nil
}

func writeConfig() error {
	var pth string
	var err error
	if *configFile != `` {
		pth = *configFile
	} else {
		// default
		if pth, err = defaultConfigPath(); err != nil {
			return fmt.Errorf("something wrong with config path: %v", err)
		}
	}
	var b []byte
	b, err = json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("JSON marshal failure: %v", err)
	}
	return ioutil.WriteFile(pth, b, 0600)
}

func readConfig() error {
	// Build config path
	var pth string
	var err error
	if *configFile != `` {
		pth = *configFile
	} else {
		// default
		if pth, err = defaultConfigPath(); err != nil {
			return err
		}
		// If it doesn't exist, make it
		if _, err := os.Stat(pth); os.IsNotExist(err) {
			writeConfig()
		}
	}
	// Now read it out
	contents, err := ioutil.ReadFile(pth)
	if err != nil {
		return err
	}
	return json.Unmarshal(contents, &cfg)
}

func main() {
	var err error
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

	// If the command was "init", we actually handle that *before* reading the
	// config file, because we're going to re-write a new config.
	if cmd == "init" {
		// Make sure we have a single argument
		if len(args) != 1 {
			log.Fatalf("Usage: zk init <path>")
		}
		root := args[0]
		// First we attempt to open an existing ZK if it's pre-populated
		if z, err = zk.NewZK(root); err != nil {
			// NewZK failed, we better call init
			if err := zk.InitZK(root); err != nil {
				// If both calls failed, something bad has happened
				log.Fatalf("Couldn't initialize new zk: %v", err)
			}
		}
		// If we got this far, one of the calls succeeded.
		cfg.ZKRoot = root
		if err := writeConfig(); err != nil {
			log.Fatalf("Couldn't write-back config: %v", err)
		}
		return
	}

	if err := readConfig(); err != nil {
		log.Fatalf("Failed to read config: %v", err)
	}

	if z, err = zk.NewZK(cfg.ZKRoot); err != nil {
		log.Fatal(err)
	}
	defer z.Close()

	switch cmd {
	case "show", "s":
		showNote(args)
	case "new", "n":
		newNote(args)
	case "up", "u":
		if cfg.CurrentNoteId != 0 {
			if md, err := z.GetNoteMeta(cfg.CurrentNoteId); err != nil {
				log.Fatalf("Couldn't get info about current note: %v", err)
			} else {
				changeLevel(md.Parent)
				showNote([]string{})
			}
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
	case "grep":
		grep(args)
	case "tgrep":
		tgrep(args)
	case "rescan":
		z.Rescan()
	case "orphans":
		orphans(args)
	default:
		if flag.NArg() == 1 {
			id, err := strconv.Atoi(flag.Arg(0))
			if err != nil {
				log.Fatalf("couldn't parse id: %v", err)
			}
			// we've been given an argument, try to change to the specified note
			changeLevel(id)
			showNote([]string{})
		} else if flag.NArg() == 0 {
			// just show the current note
			showNote([]string{})
		} else {
			log.Fatalf("Invalid command")
		}
	}

	writeConfig()
}

func newNote(args []string) {
	var targetNote int
	var err error

	if len(args) == 0 {
		targetNote = cfg.CurrentNoteId
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

	newId, err := z.NewNote(targetNote, string(body))
	if err != nil {
		log.Fatal("couldn't create note: %v", err)
	}
	fmt.Printf("Created new note %v", newId)
}

func showNote(args []string) {
	var targetNote int
	var err error

	if len(args) == 0 {
		targetNote = cfg.CurrentNoteId
	} else if len(args) == 1 {
		targetNote, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("failed to parse specified note %v: %v", args[0], err)
		}
	} else {
		log.Fatalf("usage: zk show [note]")
	}

	note, err := z.GetNoteMeta(targetNote)
	if err != nil {
		log.Fatalf("couldn't read note: %v", err)
	}

	var subnotes []zk.NoteMeta
	for _, n := range note.Subnotes {
		sn, err := z.GetNoteMeta(n)
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
	if _, err := z.GetNoteMeta(id); err != nil {
		log.Fatalf("invalid note id %v", id)
	}

	cfg.CurrentNoteId = id
}

func addFile(args []string) {
	var err error
	var srcPath string
	target := cfg.CurrentNoteId
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
	// Add the file
	// TODO: allow the user to specify an alternate name
	if err := z.AddFile(target, srcPath, ""); err != nil {
		log.Fatalf("Failed to add file: %v", err)
	}

	// Re-read the note to update the metadata
	n, err := z.GetNote(target)
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
	target := cfg.CurrentNoteId
	if len(args) == 1 {
		target, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("can't parse id: %v")
		}
	}
	// Re-read the note to update the metadata
	n, err := z.GetNote(target)
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
	target := cfg.CurrentNoteId
	if len(args) == 1 {
		target, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("can't parse id: %v")
		}
	}
	// TODO: add editor to config
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	p, err := z.GetNoteBodyPath(target)
	if err != nil {
		log.Fatalf("Couldn't get path to note body: %v", err)
	}
	cmd := exec.Command(editor, p)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	cmd.Wait()
}

func printNote(args []string) {
	var err error
	target := cfg.CurrentNoteId
	if len(args) == 1 {
		target, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("can't parse id: %v")
		}
	}
	if note, err := z.GetNote(target); err == nil {
		fmt.Print(note.Body)
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
	if err := z.LinkNote(dst, src); err != nil {
		log.Fatalf("Failed to link %d to %d: %v", src, dst, err)
	}
}

// Unlink the specified note from the current note
func unlinkNote(args []string) {
	var err error
	target := cfg.CurrentNoteId
	var child int
	if len(args) == 1 {
		child, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("can't parse id: %v", err)
		}
	} else if len(args) == 2 {
		child, err = strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("can't parse id: %v", err)
		}
		target, err = strconv.Atoi(args[1])
		if err != nil {
			log.Fatalf("can't parse id: %v", err)
		}
	} else {
		log.Fatal("Usage: zk unlink <child> [parent] ")
	}
	if err := z.UnlinkNote(target, child); err != nil {
		log.Fatal("Failed to unlink %d from %d: %v", child, target, err)
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
	if note, err := z.GetNoteMeta(id); err == nil {
		for i := 0; i < depth; i++ {
			fmt.Printf("	")
		}
		fmt.Printf("%s\n", formatNoteSummary(note))
		for _, sn := range note.Subnotes {
			printTreeRecursive(depth+1, sn)
		}
	} else {
		log.Fatalf("Problem getting note %d in recursive tree print: %v", id, err)
	}
}

func formatNoteSummary(note zk.NoteMeta) string {
	return fmt.Sprintf("%d %s", note.Id, note.Title)
}

func grep(args []string) {
	if len(args) == 0 {
		log.Fatalf("Must give a pattern to grep for")
	}
	// Just in case somebody leaves off quotes, we'll just join all args by space
	pattern := strings.Join(args, " ")

	if c, err := z.Grep(pattern, []int{}); err != nil {
		log.Fatal(err)
	} else {
		for r := range c {
			fmt.Printf("%d [%v]: %s\n", r.Note.Id, r.Note.Title, r.Line)
		}
	}
}

func tgrep(args []string) {
	if len(args) == 0 {
		log.Fatalf("Syntax: zk tgrep [root id] <pattern>")
	}
	// Root ID is optional (current note is implied) so let's check
	root := cfg.CurrentNoteId
	if len(args) >= 2 {
		// Try to parse the first arg as an int
		if id, err := strconv.Atoi(args[0]); err == nil {
			root = id
			args = args[1:]
		}
	}
	// Just in case somebody leaves off quotes, we'll just join all args by space
	pattern := strings.Join(args, " ")

	if c, err := z.TreeGrep(pattern, root); err != nil {
		log.Fatal(err)
	} else {
		for r := range c {
			fmt.Printf("%d [%v]: %s\n", r.Note.Id, r.Note.Title, r.Line)
		}
	}
}
	


func orphans(args []string) {
	if len(args) != 0 {
		log.Fatalf("orphans command takes no arguments")
	}
	orphans := z.GetOrphans()
	for _, o := range orphans {
		fmt.Println(formatNoteSummary(o))
	}
}
