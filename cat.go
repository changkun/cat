// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	flag.CommandLine.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cat [FILE]...
Concatenate FILE(s) to standard output.

examples:
$ cat --help
$ cat ./cat.go
`)
		flag.PrintDefaults()
	}
	flag.CommandLine.SetOutput(io.Discard)
	flag.Parse()

	var errs []error
	defer func() {
		for _, err := range errs {
			if err != nil {
				fmt.Fprintf(os.Stderr, "cat: %v\n", err)
			}
		}
	}()

	switch args := flag.Args(); len(args) {
	case 0:
		_, err := io.Copy(os.Stdout, os.Stdin)
		errs = append(errs, err)
	default:
		for _, arg := range args {
			err := cat(arg, os.Stdout)
			errs = append(errs, err)
		}
	}
}

// cat catches the content from a given file path and
// writes everything to the given writer if possible.
func cat(src string, w io.Writer) error {
	src = filepath.Clean(src)

	i, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("%s: No such file or directory", src)
	}
	if i.IsDir() {
		return fmt.Errorf("%s: Is a directory", i.Name())
	}
	if i.Mode()&os.ModeSymlink != 0 {
		// According to readlinkat(2), there are only two possible
		// errors EBADF and ENOTDIR but both are not possible to occur.
		// Hence, don't mind the error here as the subsequent os.Open
		// will throw the error, too. See https://linux.die.net/man/2/readlinkat
		src, _ = os.Readlink(src)
	}

	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open %s", src)
	}
	// No need to check error here. As the (*File).Close() says that
	// only files support cancellation or double close will throw an
	// error. We are not the case.
	defer f.Close()

	_, err = io.Copy(w, f)
	return err
}
