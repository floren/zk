# zk: fast CLI note-taking 

'zk' is a tool for making and organizing hierarchical notes at the command line. It tries to stay out of your way; I wrote it because the only way I'll take notes is if it's as easy as possible.

Every note gets a unique ID assigned at creation time. Notes are organized in a tree, starting with note 0 at the root (note 0 is created automatically when you run `zk init`).

zk remembers which note you've been working in. When you run a command, zk will act on the current note if no other note id is specified.

The code to interact with zk's file structure is extracted into a library, [libzk](https://pkg.go.dev/github.com/floren/zk/libzk). This means if you hate the default CLI interface, you can write your own.

## Commands

Commands in zk can typically be abbreviated to a single letter. zk offers the following commands:

### Browsing & Viewing Notes

* `show` (`s`): show the current note's title and any subnote titles. If a note id is given as an argument, it will show that note instead.
* `<id>`: set the current note to the given id
* `up` (`u`): move up one level in the tree. If a note is linked into multiple places, this may be confusing!
* `print` (`p`): print out the current note or the specified note ID.
* `tree` (`t`): show the full note tree from the root (0) or from the specified ID.
* `grep`: find notes containing the specified regular expression, e.g. `zk grep foo` or `zk grep "foo.+bar"`.
* `tgrep`: file notes containing the specified regular expression under the current or specified note, e.g. `zk tgrep 17 foobar` to find "foobar" in note 17 or its sub-notes.

Running `zk` with no arguments will list the title of the current note and its immediate sub-notes.

### Creating and Editing Notes
* `new` (`n`): create a new note under the current note or under the specified note ID. zk will prompt you for a title and any additional text you want to enter into the note at this time.
* `edit` (`e`): edit the current note (or specify a note id as an argument to edit a different one). Uses the $EDITOR variable to determine which editor to run.
* `append` (`a`): append to the current note (or specified note id). Reads from standard input.
* `link`: link a note as a sub-note of another. `zk link 22 3` will make note 22 a sub-note of note 3. `zk link 22` will make note 22 a sub-note of the *current* note.
* `unlink`: unlink a sub-note from the current note, e.g. `zk unlink 22`. As with the link command, `zk unlink 22 3` will *remove* 22 as a sub-note of note 3.

### Aliases
* `alias`: define a new alias, a human-friendly name for a particular note, e.g. `zk alias 7 todo`; you can then use "todo" in place of "7" in future commands.
* `unalias`: remove an alias, e.g. `zk unalias todo`.
* `aliases`: list existing aliases.

### Misc.
* `init`: takes a file path as an argument, sets up a zk in that directory. If the directory already contains zk files, simply sets that as the new default.
* `orphans`: list notes with no parents (excluding note 0). Unlinking a note from the tree entirely makes it an "orphan" and hides it; this lets you see what has been orphaned.
* `rescan`: attempts to re-derive the state from the contents of the zk directory. Shouldn't be necessary, but try this if commands like `zk tree` look weird.

## Installation and setup

Fetch and build the code; you may also need to copy the binary somewhere if $GOPATH/bin isn't in your path:

	go get github.com/floren/zk

Initialize your zk directory:

	zk init ~/zk

## User Guide / Examples

After running zk init, you'll have a top-level note and nothing else. We can see the current state like so:

	$ zk
	0 Top Level

We'll create a new note which will contain all information about my Go projects:

	$ zk n
	Enter note; the first line will be the title. Ctrl-D when done.
	Go hacking
	Notes about my Go programming stuff goes under this
	^D

The very first line I entered, "Go hacking", becomes the note title; the remainder is body. If we run 'zk' again, we'll see the new note:

	$ zk
	0 Top Level
		1 Go hacking

Note 0 is still the current note; we can set the current note to the new note like so:

	$ zk 1
	1 Go hacking

We can now create a new note in this Go category:

	$ zk n
	Enter note; the first line will be the title. Ctrl-D when done.
	zk
	
	Notes about my note-taking system (so meta)

	$ zk
	1 Go hacking
		2 zk

The "up" command (abbreviated 'u') takes you up one level in the tree, or you can specify a note ID number directly to go to it:

	$ zk u
	0 Top Level
		1 Go hacking

	$ zk 2
	2 zk

	$ zk u
	1 Go hacking
		2 zk

	$ zk u
	0 Top Level
		1 Go hacking

	$ zk 1
	1 Go hacking
		2 zk

	$ zk 0
	0 Top Level
		1 Go hacking

I'll create another note under the top-level, make another note under *that* note, then use the 'tree' command to see all notes:

	$ zk n 0
	Enter note; the first line will be the title. Ctrl-D when done.
	Foo

	$ zk
	0 Top Level
		1 Go hacking
		3 Foo

	$ zk 3
	3 Foo

	$ zk n
	Enter note; the first line will be the title. Ctrl-D when done.
	Bar

	$ zk t
	0 Top Level
		1 Go hacking
			2 zk
		3 Foo
			4 Bar

Notes are never deleted, because a note can appear as the child of multiple other notes; deleting the actual file would leave them hanging. It is, however, possible to 'unlink' a child from the current note so it will not appear any more. This makes it an "orphan"; use `zk orphans` to list orphaned notes.

## Development

The implementation of zk is split out into a library, [libzk](https://pkg.go.dev/github.com/floren/zk/libzk). You first init an (empty) directory:

```
// This will create a default top-level note 0
libzk.InitZK("/path/to/rootdir")
```

Then you can call NewZK and use it:

```
	var z *ZK
	if z, err = NewZK("/path/to/rootdir"); err != nil {
		log.Fatal(err)
	}

	// Create a note as a sub-note of note 0
	if err = z.NewNote(0, "Testing\n"); err != nil {
		log.Fatal(err)
	}

	// Now make sure it is listed as a child of note 0
	var md NoteMeta
	if md, err = z.GetNoteMeta(0); err != nil {
		log.Fatal(err)
	}
	if len(md.Subnotes) != 1 {
		log.Fatalf("Wrong number of subnotes on note 0, got %d should be 1", len(md.Subnotes))
	}
```
