package tempfile

// TODO: move into gokit?

import (
	"io/ioutil"
	"os"
)

func New(pattern string) (*os.File, func(), error) {
	f, err := ioutil.TempFile("", pattern)
	if err != nil {
		return nil, func() {}, err
	}

	return f, func() {
		f.Close()

		os.Remove(f.Name())
	}, nil
}
