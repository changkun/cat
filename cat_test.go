// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"runtime"
	"sync"
	"testing"
)

func TestMainProg(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	tests := []struct {
		Name string
		Args []string
		Want string
		Skip bool
	}{
		{"cat", []string{"testdata/b.md"}, "world", false},
		{"cat", []string{"testdata/d.txt"}, "cat: cannot open testdata/none.txt\n", runtime.GOOS == "windows"},
		{"cat", []string{"-abc"}, `Usage: cat [FILE]...
Concatenate FILE(s) to standard output.

examples:
$ cat --help
$ cat ./cat.go
`, false},
	}
	for _, tt := range tests {
		if tt.Skip {
			continue
		}

		flag.CommandLine = flag.NewFlagSet(tt.Name, flag.ContinueOnError)
		os.Args = append([]string{tt.Name}, tt.Args...)
		t.Log(os.Args)

		got := captureOutput(func() { main() })
		t.Log(got)
		if tt.Want != got {
			t.Errorf("unexpected output: got %v want %v", got, tt.Want)
		}
	}
}

func TestCat(t *testing.T) {
	read := func(fpath string) []byte {
		b, err := os.ReadFile(fpath)
		if err != nil {
			panic(err)
		}
		return b
	}

	t.Run("success", func(t *testing.T) {
		tests := []struct {
			fpath string
			want  []byte
			skip  bool
		}{
			{
				fpath: "./testdata/a.txt",
				want:  read("./testdata/a.txt"),
			},
			{
				fpath: "./testdata/b.md",
				want:  read("./testdata/b.md"),
			},
			{
				// c.txt is a symbolic link to a.txt
				fpath: "./testdata/c.txt",
				want:  read("./testdata/a.txt"),
				skip:  runtime.GOOS == "windows", // symbolic link does not work on Windows.
			},
			{
				fpath: "./testdata/x.png",
				want:  read("./testdata/x.png"),
			},
		}

		for _, tt := range tests {
			if tt.skip {
				continue
			}

			w := newCompleteWriter()
			err := cat(tt.fpath, w)
			if err != nil {
				t.Fatalf("failed to cat file %s: %v", tt.fpath, err)
			}

			if !bytes.Equal(tt.want, w.Bytes()) {
				t.Fatalf("cat file %s content inconsistent, got %s, want %s", tt.fpath, w.String(), tt.want)
			}
		}
	})

	t.Run("fail", func(t *testing.T) {
		tests := []struct {
			fpath string
			w     io.Writer
			err   error
			skip  bool
		}{
			{
				fpath: "none.txt",
				w:     newIncompleteWriter(),
				err:   errors.New("none.txt: No such file or directory"),
			},
			{
				fpath: "testdata",
				w:     newIncompleteWriter(),
				err:   errors.New("testdata: Is a directory"),
			},
			{
				fpath: "testdata/a.txt",
				w:     newFaultyWriter(),
				err:   errors.New("unexpected EOF"),
			},
			{
				// c.txt is a symbolic link to none.txt, which does not exist
				fpath: "testdata/d.txt",
				w:     newIncompleteWriter(),
				err:   errors.New("cannot open testdata/none.txt"),
				skip:  runtime.GOOS == "windows", // symbolic link does not work on Windows.
			},
		}

		for _, tt := range tests {
			if tt.skip {
				continue
			}

			err := cat(tt.fpath, tt.w)
			if err == nil {
				t.Fatalf("%s: expect cat to fail, but successed", tt.fpath)
			}
			if err.Error() != tt.err.Error() {
				t.Fatalf("%s: unexpected error, got %v want %v", tt.fpath, err, tt.err)
			}
		}
	})
}

type completeWriter struct{ buf []byte }

func newCompleteWriter() *completeWriter { return &completeWriter{buf: []byte{}} }
func (c *completeWriter) Write(b []byte) (int, error) {
	c.buf = append(c.buf, b...)
	return len(b), nil
}
func (c *completeWriter) Bytes() []byte  { return c.buf }
func (c *completeWriter) String() string { return string(c.buf) }

type incompleteWriter struct{ buf []byte }

func newIncompleteWriter() *incompleteWriter {
	return &incompleteWriter{buf: []byte{}}
}

func (c *incompleteWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, io.EOF
	}

	c.buf = append(c.buf, b[0])
	return 1, nil
}

func (c *incompleteWriter) Bytes() []byte {
	return c.buf
}

func (c *incompleteWriter) String() string {
	return string(c.buf)
}

type faultyWriter struct{}

func newFaultyWriter() *faultyWriter                { return &faultyWriter{} }
func (f *faultyWriter) Write(b []byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func captureOutput(f func()) string {
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	stdout := os.Stdout
	stderr := os.Stderr
	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
		log.SetOutput(os.Stderr)
	}()
	os.Stdout = writer
	os.Stderr = writer
	log.SetOutput(writer)
	out := make(chan string)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		var buf bytes.Buffer
		wg.Done()
		io.Copy(&buf, reader)
		out <- buf.String()
	}()
	wg.Wait()
	f()
	writer.Close()
	return <-out
}

func BenchmarkCat(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cat("./testdata/a.txt", io.Discard)
	}
}
