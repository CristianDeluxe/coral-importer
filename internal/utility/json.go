package utility

import (
	"bufio"
	"io"
	"os"

	"github.com/mailru/easyjson"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func NewJSONWriter(fileName string) (*JSONWriter, error) {
	f, err := os.Create(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "could not create file for writing")
	}

	w := bufio.NewWriter(f)

	return &JSONWriter{
		f:        f,
		w:        w,
		filename: fileName,
	}, nil
}

type JSONWriter struct {
	f         *os.File
	w         *bufio.Writer
	documents uint64
	filename  string
}

func (c *JSONWriter) Write(doc easyjson.Marshaler) error {
	if _, err := easyjson.MarshalToWriter(doc, c.w); err != nil {
		return errors.Wrap(err, "could not marshal output")
	}

	if _, err := c.w.WriteString("\n"); err != nil {
		return errors.Wrap(err, "could not write newline")
	}

	c.documents++

	return nil
}

func (c *JSONWriter) Close() error {
	if err := c.w.Flush(); err != nil {
		return errors.Wrap(err, "could not flush")
	}

	if err := c.f.Close(); err != nil {
		return errors.Wrap(err, "could not close file")
	}

	logrus.WithField("documents", c.documents).WithField("fileName", c.filename).Info("wrote documents")

	return nil
}

type JSONReaderFn func(line int, data []byte) error

func ReadJSON(fileName string, fn JSONReaderFn) error {
	f, err := os.Open(fileName)
	if err != nil {
		return errors.Wrap(err, "could not open file for reading")
	}
	defer f.Close()

	// Setup the scanner.
	r := bufio.NewReader(f)

	// Keep track of the processed lines.
	lines := 0

	// Start reading the stories line by line from the file.
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return errors.Wrap(err, "couldn't read the line")
		}

		// Increment the line count.
		lines++

		// Send the input to a processor.
		if err := fn(lines, []byte(line)); err != nil {
			return errors.Wrap(err, "could not operate on the line")
		}
	}

	return nil
}
