// Copyright 2024 Ross Light
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//		 https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"zombiezen.com/go/gregorian"
)

type Task struct {
	Name        string          `json:"name"`
	CreatedAt   time.Time       `json:"created_at"`
	DueAt       *time.Time      `json:"due_at"`
	DueOn       *gregorian.Date `json:"due_on"`
	Description string          `json:"notes"`
}

func main() {
	dryRun := flag.Bool("n", false, "dry run")
	verbose := flag.Bool("v", false, "verbose")
	index := flag.Bool("index", false, "whether to generate an index file")
	actionable := flag.Bool("actionable", true, "whether to include checkboxes")
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(64)
	}
	outputDir := flag.Arg(0)
	if outputDir == "" {
		outputDir = "."
	}
	var output fileWriter
	switch {
	case *dryRun:
		output = &logWriter{
			dir: outputDir,
			w:   nopWriter{},
		}
	case *verbose:
		output = &logWriter{
			dir: outputDir,
			w:   dirWriter(outputDir),
		}
	default:
		output = dirWriter(outputDir)
	}

	s := bufio.NewScanner(os.Stdin)
	ok := true
	var tasks []*Task
	for lineno := 1; s.Scan(); lineno++ {
		task := new(Task)
		if err := json.Unmarshal(s.Bytes(), task); err != nil {
			fmt.Fprintf(os.Stderr, "asana-to-md: line %d: %v\n", lineno, err)
			ok = false
			continue
		}
		tasks = append(tasks, task)
	}
	if !*dryRun && !ok {
		// Since writing is not idempotent (we append),
		// abort early if we're not doing a dry run and we didn't read our input correctly.
		os.Exit(1)
	}
	writeTasks(output, tasks, &writeOptions{
		loc:            time.Local,
		index:          *index,
		omitCheckboxes: !*actionable,
		reportError: func(err error) {
			fmt.Fprintf(os.Stderr, "asana-to-md: %v\n", err)
			ok = false
		},
	})
	if !ok {
		os.Exit(1)
	}
}

type writeOptions struct {
	loc            *time.Location
	index          bool
	omitCheckboxes bool
	reportError    func(error)
}

func writeTasks(w fileWriter, tasks []*Task, opts *writeOptions) {
	grouped := groupTasksByMinute(opts.loc, tasks)
	buf := new(bytes.Buffer)
	for _, key := range sortedKeys(grouped) {
		tasks := grouped[key]
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
		})
		buf.Reset()
		for _, t := range tasks {
			buf.WriteString("- ")
			if !opts.omitCheckboxes {
				buf.WriteString("[ ] ")
			}
			fmt.Fprintf(buf, "%s #inbox", t.Name)
			switch {
			case t.DueAt != nil:
				fmt.Fprintf(buf, " 📅 %s", t.DueAt.In(opts.loc).Format("2006-01-02"))
			case t.DueOn != nil:
				fmt.Fprintf(buf, " 📅 %v", t.DueOn)
			}
			buf.WriteString("\n")
			if t.Description != "" {
				buf.WriteString("\n")
				buf.WriteString(t.Description)
				buf.WriteString("\n\n")
			}
		}

		if err := w.writeFile(key+".md", buf.Bytes()); err != nil && opts.reportError != nil {
			opts.reportError(err)
		}
	}

	if opts.index {
		buf.Reset()
		generateIndex(buf, grouped)
		indexName := time.Now().In(opts.loc).Format(filenameTimeFormat)
		if err := w.writeFile(indexName+".md", buf.Bytes()); err != nil && opts.reportError != nil {
			opts.reportError(err)
		}
	}
}

const filenameTimeFormat = "200601021504"

func groupTasksByMinute(loc *time.Location, tasks []*Task) map[string][]*Task {
	grouped := make(map[string][]*Task)
	for _, t := range tasks {
		key := t.CreatedAt.In(loc).Format(filenameTimeFormat)
		grouped[key] = append(grouped[key], t)
	}
	for key := range grouped {
		tasks := grouped[key]
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
		})
	}
	return grouped
}

func generateIndex(buf *bytes.Buffer, grouped map[string][]*Task) {
	buf.WriteString("---\n" +
		"tags:\n" +
		"- inbox\n" +
		"---\n\n")
	for _, key := range sortedKeys(grouped) {
		for _, task := range grouped[key] {
			fmt.Fprintf(buf, "- [%s](%s.md)\n", task.Name, key)
		}
	}
}

type fileWriter interface {
	writeFile(name string, data []byte) error
}

type dirWriter string

func (dir dirWriter) writeFile(name string, data []byte) error {
	if !fs.ValidPath(name) || name == "." {
		return &fs.PathError{
			Op:   "write",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}
	fsPath := filepath.Join(string(dir), filepath.FromSlash(name))

	if err := os.MkdirAll(filepath.Dir(fsPath), 0o777); err != nil {
		return err
	}

	f, err := os.OpenFile(fsPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o666)
	if err != nil {
		return err
	}
	if needsBlankLine(f) {
		data = append([]byte("\n\n"), data...)
	}
	var errs [2]error
	_, errs[0] = f.Write(data)
	errs[1] = f.Close()
	for _, err := range errs {
		if err != nil {
			return &fs.PathError{
				Op:   "write",
				Path: name,
				Err:  err,
			}
		}
	}
	return nil
}

func needsBlankLine(r io.ReadSeeker) bool {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil || size == 0 {
		// If the file is not seekable, assume we're appending to something special
		// and don't add blank line.
		// If the file is empty, we don't need the blank line.
		return false
	}
	const wantEnding = "\n\n"
	if size < int64(len(wantEnding)) {
		// Don't bother reading if we have some content
		// but it's not long enough for it to end in a blank line.
		return true
	}

	// See if we end with a newline. If we fail here, assume we need a blank line.
	if _, err := r.Seek(-int64(len(wantEnding)), io.SeekEnd); err != nil {
		return true
	}
	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return true
	}
	return string(buf[:]) != wantEnding
}

type logWriter struct {
	dir string
	w   fileWriter
}

func (w *logWriter) writeFile(name string, data []byte) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{
			Op:   "write",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}
	fsPath := filepath.Join(string(w.dir), filepath.FromSlash(name))

	info, err := os.Stat(fsPath)
	var marker string
	if err == nil && info.Mode().IsRegular() {
		marker = " (append)"
	}

	fmt.Printf("%s\t%d lines%s\n", fsPath, bytes.Count(data, []byte("\n")), marker)
	return w.w.writeFile(name, data)
}

func sortedKeys[K ordered, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}

type ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64 | ~string
}

type nopWriter struct{}

func (nopWriter) writeFile(name string, data []byte) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{
			Op:   "write",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}
	return nil
}
