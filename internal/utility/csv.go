package utility

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
)

type CSVReaderFn func(line int, fields []string) error

func ReadCSV(fileName string, fieldsPerRecord int, fn CSVReaderFn) error {
	f, err := os.Open(fileName)
	if err != nil {
		return errors.Wrap(err, "could not open file for reading")
	}
	defer f.Close()

	r := csv.NewReader(bufio.NewReader(f))
	r.FieldsPerRecord = fieldsPerRecord
	r.TrimLeadingSpace = true

	var line int

	// Try to read the first line, we expect that it should be a header row.
	fields, err := r.Read()
	if err != nil {
		return errors.Wrap(err, "could not read CSV")
	}

	if strings.ToLower(fields[0]) != "id" {
		return fmt.Errorf("expected header row")
	}

	for {
		fields, err = r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return errors.Wrap(err, "could not read CSV")
		}

		if err := fn(line, fields); err != nil {
			return errors.Wrap(err, "reading failed")
		}

		line++
	}

	return nil
}
