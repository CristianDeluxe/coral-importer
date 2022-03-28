package warnings

import "sync"

func NewWarning() *Warning {
	return &Warning{
		once: &sync.Once{},
		m:    &sync.Map{},
	}
}

type Warning struct {
	once *sync.Once
	m    *sync.Map
}

// OnceWith will only allow the function to execute if the key has not been
// seen yet.
func (w *Warning) OnceWith(fn func(), key string) {
	// Load the empty struct or store a new one. If it was loaded (and not
	// stored) then we have already seen this key and it should be skipped.
	if _, ok := w.m.LoadOrStore(key, struct{}{}); ok {
		return
	}

	fn()
}

// Once will only allow the function to execute if it has not before.
func (w *Warning) Once(fn func()) {
	w.once.Do(fn)
}
