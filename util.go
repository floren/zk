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
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

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
