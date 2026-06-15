package utility

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"

	"github.com/mailru/easyjson"
	"github.com/pkg/errors"
)

// Writer interface defines methods for writing JSON data.
type Writer interface {
	Write(doc easyjson.Marshaler) error
	Close() error
}

// nopJSONWriter is a no-op JSON writer used for dry runs.
type nopJSONWriter struct{}

// Write does nothing in nopJSONWriter.
func (d *nopJSONWriter) Write(doc easyjson.Marshaler) error { return nil }

// Close does nothing in nopJSONWriter.
func (d *nopJSONWriter) Close() error { return nil }

// NewJSONWriter creates a new JSON writer to write data to the specified file.
// If dryRun is true, it will use nopJSONWriter instead of writing to a file.
func NewJSONWriter(dryRun bool, fileName string) (Writer, error) {
	if dryRun {
		return &nopJSONWriter{}, nil
	}

	// Create the destination file for writing
	dest, err := os.Create(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "could not create file for writing")
	}

	// Buffer the writes for efficiency
	w := bufio.NewWriter(dest)

	// Return JSONWriter instance with a buffered writer
	return &JSONWriter{
		f: dest,
		w: w,
	}, nil
}

// JSONWriter is responsible for writing JSON data to a file.
type JSONWriter struct {
	f io.WriteCloser
	w *bufio.Writer
}

// Write writes marshalled JSON data to the buffer and then writes a newline.
func (c *JSONWriter) Write(doc easyjson.Marshaler) error {
	// Marshal the document into JSON and write to the buffer
	if _, err := easyjson.MarshalToWriter(doc, c.w); err != nil {
		return errors.Wrap(err, "could not marshal output")
	}

	// Write a newline after the marshalled JSON
	if _, err := c.w.WriteRune('\n'); err != nil {
		return errors.Wrap(err, "could not write newline")
	}

	return nil
}

// Close flushes the buffer and closes the file.
func (c *JSONWriter) Close() error {
	// Flush any remaining buffered data to the file
	if err := c.w.Flush(); err != nil {
		return errors.Wrap(err, "could not flush")
	}

	// Close the underlying file
	if err := c.f.Close(); err != nil {
		return errors.Wrap(err, "could not close file")
	}

	return nil
}

// JSONReaderFn defines a function signature for reading and processing JSON lines.
type JSONReaderFn func(line int, data []byte) error

// Line holds data for a specific line in the JSON file.
type Line struct {
	LineNumber int
	Data       []byte
}

// ReadJSONConcurrently reads a JSON file concurrently and processes each line using the provided function.
// It creates multiple goroutines based on the number of CPU cores to process the data in parallel.
func ReadJSONConcurrently(fileName string, fn JSONReaderFn) error {
	// Use the number of CPU cores available
	count := runtime.NumCPU()
	ch := make(chan Line, count)
	var wg sync.WaitGroup

	// Start concurrent workers equal to the number of CPU cores
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Goroutine panicked: %v\n", r)
				}
				wg.Done()
			}()

			for line := range ch {
				// Fix the JSON data before processing
				fixedData, err := fixJSON(line.Data)
				if err != nil {
					fmt.Printf("Error fixing JSON on line %d: %s\n", line.LineNumber, err)
					continue
				}

				// Process each line using the provided function
				if err := fn(line.LineNumber, fixedData); err != nil {
					fmt.Printf("Error parsing line %d: %s\n", line.LineNumber, err)
					continue
				}
			}
		}()
	}

	// Read the JSON file using json.Decoder
	if err := ReadJSON(fileName, func(line int, data []byte) error {
		// Send the line to the worker channel
		ch <- Line{
			LineNumber: line,
			Data:       data,
		}
		return nil
	}); err != nil {
		close(ch)
		return err
	}

	// Close the channel and wait for all workers to finish
	close(ch)
	wg.Wait()

	return nil
}

// ReadJSON reads a JSON file line by line and processes each complete JSON object.
// It buffers data to ensure complete JSON objects are processed correctly.
func ReadJSON(fileName string, fn JSONReaderFn) error {
	// Open the file for reading
	f, err := os.Open(fileName)
	if err != nil {
		return errors.Wrap(err, "could not open file for reading")
	}
	defer f.Close()

	// Create a JSON decoder
	decoder := json.NewDecoder(f)

	lines := 0

	// Read tokens until EOF
	for {
		var rawMessage json.RawMessage
		if err := decoder.Decode(&rawMessage); err != nil {
			if err == io.EOF {
				break
			}
			return errors.Wrap(err, "could not decode JSON")
		}

		lines++
		if err := fn(lines, rawMessage); err != nil {
			return errors.Wrap(err, "could not operate on the line")
		}
	}

	return nil
}

// fixJSON processes and fixes the JSON data before parsing.
// It handles MongoDB extended JSON fields like $oid and $date,
// ensures that 'user_id' is a string, 'body_history' is an array,
// and 'metadata' is an object.
func fixJSON(data []byte) ([]byte, error) {
	// Unmarshal the JSON data into a map for manipulation
	var raw map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber() // UseNumber to preserve number formats
	if err := decoder.Decode(&raw); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal JSON")
	}

	// Process the raw map to fix any issues
	fixMap(raw)

	// Marshal the fixed map back into JSON
	fixedData, err := json.Marshal(raw)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal fixed JSON")
	}

	return fixedData, nil
}

// fixMap recursively fixes issues in the JSON data map
func fixMap(m map[string]interface{}) {
	for key, value := range m {
		switch v := value.(type) {
		case map[string]interface{}:
			// Handle MongoDB $oid and $date fields
			if oid, ok := v["$oid"]; ok {
				if oidStr, ok := oid.(string); ok {
					m[key] = oidStr
				}
			} else if date, ok := v["$date"]; ok {
				if dateStr, ok := date.(string); ok {
					m[key] = dateStr
				}
			} else {
				// Recurse into nested maps
				fixMap(v)
			}
		case []interface{}:
			// Recurse into slices
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					fixMap(itemMap)
				} else if num, ok := item.(json.Number); ok {
					// Convert numbers to appropriate types
					if intVal, err := num.Int64(); err == nil {
						v[i] = intVal
					} else if floatVal, err := num.Float64(); err == nil {
						v[i] = floatVal
					}
				}
			}
		case json.Number:
			// Convert numbers to appropriate types
			if key == "user_id" {
				// Convert 'user_id' to string
				m[key] = v.String()
			} else {
				if intVal, err := v.Int64(); err == nil {
					m[key] = intVal
				} else if floatVal, err := v.Float64(); err == nil {
					m[key] = floatVal
				}
			}
		case string:
			// No action needed for strings
		default:
			// Handle other types if necessary
		}
	}

	// Additional fixes outside the loop
	// Ensure 'body_history' is an array
	if bodyHistory, ok := m["body_history"]; ok {
		if _, isArray := bodyHistory.([]interface{}); !isArray {
			m["body_history"] = []interface{}{bodyHistory}
		}
	}

	// Ensure 'metadata' is an object
	if metadata, ok := m["metadata"]; ok {
		if metadata == nil {
			m["metadata"] = map[string]interface{}{}
		} else if _, isArray := metadata.([]interface{}); isArray {
			m["metadata"] = map[string]interface{}{}
		}
	}

	// Convert 'user_id' to string if it's not already
	if userID, ok := m["user_id"]; ok {
		switch uid := userID.(type) {
		case json.Number:
			m["user_id"] = uid.String()
		case float64:
			m["user_id"] = strconv.FormatInt(int64(uid), 10)
		case int64:
			m["user_id"] = strconv.FormatInt(uid, 10)
		case int:
			m["user_id"] = strconv.Itoa(uid)
		}
	}

	// Ensure 'status_history' is an array
	if statusHistory, ok := m["status_history"]; ok {
		if _, isArray := statusHistory.([]interface{}); !isArray {
			m["status_history"] = []interface{}{statusHistory}
		}
	}
}
