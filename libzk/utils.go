package zk

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

func (z *ZK) readState() (err error) {
	p := filepath.Join(z.root, "state")
	fd, err := os.OpenFile(p, os.O_RDWR, 0700)
	if err != nil {
		return
	}
	defer fd.Close()

	dec := json.NewDecoder(fd)
	err = dec.Decode(&z.state)
	if err != nil {
		// If we couldn't decode the state, we need to try and derive it
		err = z.deriveState()
		if err != nil {
			err = fmt.Errorf("couldn't parse state or derive it: %v", err)
		}
	}
	return
}

func (z *ZK) deriveState() (err error) {
	z.state.Notes = make(map[int]NoteMeta)
	// Stat the directory
	var contents []os.FileInfo
	contents, err = ioutil.ReadDir(z.root)
	if err != nil {
		return
	}

	// Walk each numeric subdirectory and see if we can extract their info
	for i := range contents {
		if contents[i].IsDir() {
			if id, err := strconv.Atoi(contents[i].Name()); err == nil {
				var meta NoteMeta
				meta, err = z.readNoteMetadata(id)
				if err != nil {
					// oh well
					continue
				}
				z.state.Notes[id] = meta
				if id >= z.state.NextNoteId {
					z.state.NextNoteId = id + 1
				}
			}
		}
	}
	return
}

func (z *ZK) readNoteMetadata(id int) (meta NoteMeta, err error) {
	noteRoot := filepath.Join(z.root, fmt.Sprintf("%d", id))
	metaPath := filepath.Join(noteRoot, "metadata")
	fd, err := os.OpenFile(metaPath, os.O_RDWR, 0755)
	if err != nil {
		return meta, err
	}
	defer fd.Close()

	dec := json.NewDecoder(fd)
	err = dec.Decode(&meta)
	if err != nil && err != io.EOF {
		return meta, fmt.Errorf("failure parsing state file: %v", err)
	}

	files, err := ioutil.ReadDir(filepath.Join(noteRoot, "files"))
	if err != nil {
		log.Fatal(err)
	}
fileLoop:
	for _, f := range files {
		for i := range meta.Files {
			if meta.Files[i] == f.Name() {
				continue fileLoop
			}
			meta.Files = append(meta.Files, f.Name())
		}
	}

	return meta, nil
}

// infers where to write based on the metadata
func (z *ZK) writeNoteMetadata(meta NoteMeta) error {
	p := filepath.Join(z.root, fmt.Sprintf("%d", meta.Id), "metadata")
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

func (z *ZK) writeState() (err error) {
	p := filepath.Join(z.root, "state")
	fd, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		err = fmt.Errorf("Failed to open state file: %v", err)
		return
	}
	defer fd.Close()

	fd.Truncate(0)
	fd.Seek(0, 0)
	enc := json.NewEncoder(fd)
	err = enc.Encode(z.state)
	if err != nil {
		err = fmt.Errorf("Failure marshalling to state file: %v", err)
	}
	fd.Sync()
	return
}
