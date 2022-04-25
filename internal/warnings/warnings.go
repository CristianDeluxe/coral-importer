package warnings

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func NewWarning(name, description string) *Warning {
	return &Warning{
		name:        name,
		description: description,
		once:        &sync.Once{},
		m:           &sync.Map{},
	}
}

type Warning struct {
	name        string
	description string

	once        *sync.Once
	m           *sync.Map
	occurrences int32
}

// OnceWith will only allow the function to execute if the key has not been
// seen yet.
func (w *Warning) OnceWith(fn func(), key string) {
	// Add to the number of occurrences.
	atomic.AddInt32(&w.occurrences, 1)

	// Load the empty struct or store a new one. If it was loaded (and not
	// stored) then we have already seen this key and it should be skipped.
	if _, ok := w.m.LoadOrStore(key, struct{}{}); ok {
		return
	}

	fn()
}

// Once will only allow the function to execute if it has not before.
func (w *Warning) Once(fn func()) {
	// Add to the number of occurrences.
	atomic.AddInt32(&w.occurrences, 1)

	w.once.Do(fn)
}

func (w *Warning) String() string {
	return fmt.Sprintf("%s: %s", w.name, w.description)
}

func (w *Warning) Occurrences() int32 {
	return atomic.LoadInt32(&w.occurrences)
}

func (w *Warning) Keys() []string {
	keys := make([]string, 0)
	w.m.Range(func(key, value interface{}) bool {
		if str, ok := key.(string); ok {
			keys = append(keys, str)
		}

		return true
	})

	return keys
}
