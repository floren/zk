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
* `up` (`u`): move up a level in the hierarchy of notes
* `<id>`: set the current note to the given id
* `init`: set up your ~/zk directory. Only run this once!