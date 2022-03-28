package utility

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/coralproject/coral-importer/internal/utility/counter"
	"github.com/pkg/errors"
)

func Exists(fileName string) bool {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return false
	}

	return true
}

func NewLineCounter(title, sourceFileName string) (*counter.Counter, error) {
	fmt.Println(title)
	lines, err := CountLines(sourceFileName)
	if err != nil {
		return nil, errors.Wrap(err, "could not count users file")
	}

	return counter.New(lines), nil
}

func CountLines(fileName string) (int, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return 0, errors.Wrap(err, "could not open file for reading")
	}
	defer f.Close()

	lines := 0
	// Should be the maximum size of a given lines worth of content.
	buf := make([]byte, 32*1024)
	lineSep := []byte{'\n'}

	for {
		c, err := f.Read(buf)
		lines += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			if lines == 0 && c >= 0 {
				return 0, errors.New("expected file to end with a newline")
			}

			return lines, nil
		case err != nil:
			return lines, err
		}
	}
}
