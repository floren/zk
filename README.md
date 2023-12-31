# zk: fast CLI note-taking 

'zk' is a tool for making and organizing hierarchical notes at the command line. It tries to stay out of your way; I wrote it because the only way I'll take notes at all is if it's as easy as possible.

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
* `rescan`: attempts to re-derive the state from the contents of the zk directory. Sometimes you'll need to run this if you've changed the title (the first line) of a note.

## Installation and setup

Fetch and build the code; make sure $GOPATH/bin is in your path!

	go install github.com/floren/zk@latest

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

### Linking

A note can appear at multiple places in the tree. Suppose I have a tree that looks like this:

	$ zk t
	0 Top Level
			1 Go hacking
					2 zk
			3 Personal Projects
					4 Bellwether mouse

Since `zk` is a personal project, I'd also like it to appear as a child of note 3, so I use the `link` command:

	$ zk link 2 3
	$ zk t
	0 Top Level
			1 Go hacking
					2 zk
			3 Personal Projects
					4 Bellwether mouse
					2 zk

If I decide that in fact, `zk` is so much a personal project that I don't even want to see it under "Go hacking" any more, I can use the `unlink` command:

	$ zk unlink 2 1
	$ zk t
	0 Top Level
			1 Go hacking
			3 Personal Projects
					4 Bellwether mouse
					2 zk

Perhaps I now realize that "Go hacking" is a silly category to have; if I unlink note 1 from note 0, it will be completely unlinked ("orphaned"). It will no longer appear in the tree.

	$ zk unlink 1 0
	$ zk t
	0 Top Level
			3 Personal Projects
					4 Bellwether mouse
					2 zk
	$ zk orphans
	1 Go hacking

There are several advantages to unlinking notes rather than deleting them:

- I can refer to "note 1" in other notes and still view it at any time, because it still exists.
- I can reinstate it into the tree whenever I wish with a `link` command.
- It will still be included in `zk grep` results (you can `zk tgrep 0 <pattern>` to restrict the search to only nodes actually in the tree)

### Aliases

Aliases let you assign human-friendly names to particular notes. Suppose I have a note where I keep a list of tasks to be done:

	$ zk p 5
	TODO

	* TODO update readme

The number 5 isn't particularly conducive to memory, so I use the `alias` command to give it another name, "todo":

	$ zk alias 5 todo
	$ zk aliases
	todo → 5 TODO

I can then use the string "todo" anywhere I would have referred to the note by number:

	john@frodo:~/hacking/zk$ zk append todo
	Ctrl-D when done.
	* TODO buy milk

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

## Internals

Notes are stored in numeric directories within your zk dir:

	$ ls ~/zk
	0/     1/     2/     3/     4/     5/     state

Each note is itself a directory, containing the `body` file, the `metadata` file, and a directory named `files` containing any files you have linked with the note (experimental feature).

	$ ls ~/zk/3
	body  files  metadata

The metadata file is JSON formatted:

    {"Id":3,"Title":"Personal Projects","Subnotes":[4,2],"Files":[],"Parent":0}

Each note has one "canonical" parent. This only comes into play with using the `zk up` command, and it faces the same issues as `cd ..` does in Unix when dealing with symlinks. 
