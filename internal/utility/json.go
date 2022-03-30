package utility

import (
	"bufio"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/mailru/easyjson"
	"github.com/pkg/errors"
)

type Writer interface {
	Write(doc easyjson.Marshaler) error
	Close() error
}

type nopJSONWriter struct{}

func (d *nopJSONWriter) Write(doc easyjson.Marshaler) error { return nil }

func (d *nopJSONWriter) Close() error { return nil }

func NewJSONWriter(dryRun bool, fileName string) (Writer, error) {
	if dryRun {
		return &nopJSONWriter{}, nil
	}

	dest, err := os.Create(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "could not create file for writing")
	}

	w := bufio.NewWriter(dest)

	return &JSONWriter{
		f: dest,
		w: w,
	}, nil
}

type JSONWriter struct {
	f io.WriteCloser
	w *bufio.Writer
}

func (c *JSONWriter) Write(doc easyjson.Marshaler) error {
	if _, err := easyjson.MarshalToWriter(doc, c.w); err != nil {
		return errors.Wrap(err, "could not marshal output")
	}

	if _, err := c.w.WriteRune('\n'); err != nil {
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

type JSONReaderFn func(line int, data []byte) error

type Line struct {
	LineNumber int
	Data       []byte
}

func ReadJSONConcurrently(fileName string, fn JSONReaderFn) error {
	count := runtime.NumCPU()
	ch := make(chan Line, count)
	var wg sync.WaitGroup

	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			for line := range ch {
				if err := fn(line.LineNumber, line.Data); err != nil {
					panic(err)
				}
			}

			wg.Done()
		}()
	}

	if err := ReadJSON(fileName, func(line int, data []byte) error {

		ch <- Line{
			LineNumber: line,
			Data:       data,
		}

		return nil
	}); err != nil {
		return err
	}

	close(ch)
	wg.Wait()

	return nil
}

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

		// We're reading JSON, and if the document is less than or equal to two
		// characters there is no content to read!
		if len(line) <= 2 {
			continue
		}

		// Send the input to a processor.
		if err := fn(lines, []byte(line)); err != nil {
			return errors.Wrap(err, "could not operate on the line")
		}
	}

	return nil
}
