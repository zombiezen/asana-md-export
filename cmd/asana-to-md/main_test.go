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
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"zombiezen.com/go/gregorian"
)

const fakeFileMode fs.FileMode = 0o644

func TestWriteTasks(t *testing.T) {
	tz := time.FixedZone("America/Los_Angeles", -8*60*60)
	tests := []struct {
		name  string
		tasks []*Task
		want  fstest.MapFS
	}{
		{
			name: "Empty",
			want: fstest.MapFS{},
		},
		{
			name: "SingleTask",
			tasks: []*Task{
				{
					Name:      "Brush teeth",
					CreatedAt: time.Date(2024, time.January, 4, 16, 5, 0, 0, time.UTC),
				},
			},
			want: fstest.MapFS{
				"202401040805.md": {
					Data: []byte("- [ ] Brush teeth #inbox\n"),
					Mode: fakeFileMode,
				},
			},
		},
		{
			name: "DueDate",
			tasks: []*Task{
				{
					Name:      "Brush teeth",
					CreatedAt: time.Date(2024, time.January, 4, 16, 5, 0, 0, time.UTC),
					DueOn:     newDate(2024, time.January, 4),
				},
			},
			want: fstest.MapFS{
				"202401040805.md": {
					Data: []byte("- [ ] Brush teeth #inbox 📅 2024-01-04\n"),
					Mode: fakeFileMode,
				},
			},
		},
		{
			name: "Multiple",
			tasks: []*Task{
				{
					Name:      "Brush teeth",
					CreatedAt: time.Date(2024, time.January, 4, 16, 5, 0, 0, time.UTC),
				},
				{
					Name:      "Buy train ticket",
					CreatedAt: time.Date(2024, time.January, 4, 1, 30, 20, 0, time.UTC),
				},
				{
					Name:      "Set alarm",
					CreatedAt: time.Date(2024, time.January, 4, 1, 30, 10, 0, time.UTC),
				},
			},
			want: fstest.MapFS{
				"202401040805.md": {
					Data: []byte("- [ ] Brush teeth #inbox\n"),
					Mode: fakeFileMode,
				},
				"202401031730.md": {
					Data: []byte(
						"- [ ] Set alarm #inbox\n" +
							"- [ ] Buy train ticket #inbox\n"),
					Mode: fakeFileMode,
				},
			},
		},
		{
			name: "Multiple",
			tasks: []*Task{
				{
					Name:      "Buy train ticket",
					CreatedAt: time.Date(2024, time.January, 4, 1, 30, 20, 0, time.UTC),
				},
				{
					Name:        "Set alarm",
					CreatedAt:   time.Date(2024, time.January, 4, 1, 30, 10, 0, time.UTC),
					Description: "Like really early.\nI hope you're ready.",
				},
			},
			want: fstest.MapFS{
				"202401031730.md": {
					Data: []byte(
						"- [ ] Set alarm #inbox\n" +
							"\nLike really early.\nI hope you're ready.\n\n" +
							"- [ ] Buy train ticket #inbox\n"),
					Mode: fakeFileMode,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := make(fstest.MapFS)
			writeTasks(mapWriter{got}, test.tasks, &writeOptions{
				loc: tz,
				reportError: func(err error) {
					t.Error(err)
				},
			})
			if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("written files (-want +got):\n%s", diff)
			}
		})
	}
}

func newDate(year int, month time.Month, day int) *gregorian.Date {
	d := gregorian.NewDate(year, month, day)
	return &d
}

type mapWriter struct {
	fs fstest.MapFS
}

func (w mapWriter) writeFile(name string, data []byte) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{
			Op:   "write",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}

	f := w.fs[name]
	if f == nil {
		f = &fstest.MapFile{
			Mode: fakeFileMode,
		}
		w.fs[name] = f
	} else if !f.Mode.IsRegular() {
		return &fs.PathError{
			Op:   "write",
			Path: name,
			Err:  errors.New("not a file"),
		}
	}
	f.Data = append(f.Data, data...)
	return nil
}
