package mapper

import "strings"

func newGetter(path string) *getter {
	parts := strings.Split(path, ".")
	return &getter{
		len:   len(parts),
		parts: parts,
	}
}

type getter struct {
	len   int
	parts []string
}

func (g *getter) Get(obj map[string]interface{}) (string, bool) {
	current := obj

	for depth, part := range g.parts {
		element, ok := current[part]
		if !ok {
			return "", false
		}

		// If we're at the end, then we should check that the element is a string.
		if depth+1 == g.len {
			value, ok := element.(string)
			if !ok {
				return "", false
			}

			return value, true
		}

		// Otherwise, we should check to see that this is a map.
		value, ok := element.(map[string]interface{})
		if !ok {
			return "", false
		}

		// And advance the pointer.
		current = value
	}

	return "", false
}
