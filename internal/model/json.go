package model

import "encoding/json"

// Marshal writes indented, stable JSON with a trailing newline.
// encoding/json sorts map keys. Callers sort slices before marshaling
// documents that contain them.
func Marshal(v any) ([]byte, error) {
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	buf = append(buf, '\n')
	return buf, nil
}
