# zk: fast CLI note-taking 

'zk' is a tool for making and organizing hierarchical notes at the command line. It tries to stay out of your way; I wrote it because the only way I'll take notes is if it's as easy as possible.

Every note gets a unique ID assigned at creation time. Notes are organized in a tree, starting with note 0 at the root (note 0 is created automatically when you run `zk init`).

zk remembers which note you've been working in. When you run a command, zk will act on the current note if no other note id is specified.

## Commands

Commands in zk can typically be abbreviated to a single letter. zk offers the following commands:

* `show` (`s`): show the current note's title and any subnote titles. If a note id is given as an argument, it will show that note instead
* `new` (`n`): create a new note under the current note. zk will prompt you for a title and any additional text you want to enter into the note at this time
* `edit` (`e`): edit the current note (or specify a note id as an argument to edit a different one). Uses the $EDITOR variable to determine which editor to run.
* `print` (`p`): print out the current note or the specified note ID
* `delete`: delete the specified note
* `up` (`u`): move up a level in the hierarchy of notes
* `<id>`: set the current note to the given id
* `init`: set up your ~/zk directory. Only run this once!

## Installation and setup

Fetch and build the code; you may also need to copy the binary somewhere if $GOPATH/bin isn't in your path:

	go get github.com/floren/zk

Initialize your zk directory:

	zk init

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

Notes are never deleted, because a note can appear as the child of multiple other notes; deleting the actual file would leave them hanging. It is, however, possible to 'unlink' a child from the current note so it will not appear any more.

