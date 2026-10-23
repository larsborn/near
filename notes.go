package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Notes live in a hidden JSON file inside each directory, mapping an
// entry name to its note text. The file is not shown in the panel.

func loadNotes(dir, notesFile string) map[string]string {
	notes := map[string]string{}
	data, err := os.ReadFile(filepath.Join(dir, notesFile))
	if err != nil {
		return notes
	}
	_ = json.Unmarshal(data, &notes)
	return notes
}

func saveNotes(dir, notesFile string, notes map[string]string) error {
	path := filepath.Join(dir, notesFile)
	for k, v := range notes {
		if v == "" {
			delete(notes, k)
		}
	}
	if len(notes) == 0 {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	// Sorted keys keep the file stable for version control.
	keys := make([]string, 0, len(notes))
	for k := range notes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(notes))
	for _, k := range keys {
		ordered[k] = notes[k]
	}
	data, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return err
	}
	hideFile(path)
	return nil
}

// moveNote transfers the note of oldName in srcDir to newName in dstDir.
func moveNote(srcDir, oldName, dstDir, newName, notesFile string) {
	src := loadNotes(srcDir, notesFile)
	text, ok := src[oldName]
	if !ok {
		return
	}
	delete(src, oldName)
	_ = saveNotes(srcDir, notesFile, src)
	dst := src
	if dstDir != srcDir {
		dst = loadNotes(dstDir, notesFile)
	}
	dst[newName] = text
	_ = saveNotes(dstDir, notesFile, dst)
}

func copyNote(srcDir, oldName, dstDir, newName, notesFile string) {
	src := loadNotes(srcDir, notesFile)
	text, ok := src[oldName]
	if !ok {
		return
	}
	dst := loadNotes(dstDir, notesFile)
	dst[newName] = text
	_ = saveNotes(dstDir, notesFile, dst)
}

func deleteNote(dir, name, notesFile string) {
	notes := loadNotes(dir, notesFile)
	if _, ok := notes[name]; !ok {
		return
	}
	delete(notes, name)
	_ = saveNotes(dir, notesFile, notes)
}
