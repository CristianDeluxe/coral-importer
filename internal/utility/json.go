package utility

import (
	"bufio"
	"os"

	"github.com/mailru/easyjson"
	"github.com/pkg/errors"
)

func NewJSONWriter(fileName string) (*JSONWriter, error) {
	f, err := os.Create(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "could not create file for writing")
	}

	w := bufio.NewWriter(f)

	return &JSONWriter{
		f: f,
		w: w,
	}, nil
}

type JSONWriter struct {
	f *os.File
	w *bufio.Writer
}

func (c *JSONWriter) Write(doc easyjson.Marshaler) error {
	if _, err := easyjson.MarshalToWriter(doc, c.w); err != nil {
		return errors.Wrap(err, "could not marshal output")
	}

	if _, err := c.w.WriteString("\n"); err != nil {
		return errors.Wrap(err, "could not write newline")
	}

	return nil
}

func (c *JSONWriter) Close() error {
	if err := c.w.Flush(); err != nil {
		return errors.Wrap(err, "could not flush")
	}

	if err := c.f.Close(); err != nil {
		return errors.Wrap(err, "could not close file")
	}

	return nil
}
