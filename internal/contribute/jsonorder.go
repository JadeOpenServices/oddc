// SPDX-License-Identifier: GPL-3.0-or-later

package contribute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// keyOrder records, for every object in a JSON document, the order its keys
// appear in, by the object's path ("" for the top, then "/key/key...").
func keyOrder(data []byte) (map[string][]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	order := map[string][]string{}
	var walk func(path string) error
	walk = func(path string) error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch token {
		case json.Delim('{'):
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name := key.(string)
				order[path] = append(order[path], name)
				if err := walk(path + "/" + name); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case json.Delim('['):
			for i := 0; decoder.More(); i++ {
				if err := walk(path + "/" + strconv.Itoa(i)); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		}
		return nil
	}
	if err := walk(""); err != nil && err != io.EOF {
		return nil, err
	}
	return order, nil
}

// encodeOrdered writes value as indented JSON (two spaces, as the catalog
// is written), objects in the key order recorded for their path, keys that
// were not there before after them in alphabetical order.
func encodeOrdered(out *bytes.Buffer, value any, path string, order map[string][]string, indent string) error {
	switch v := value.(type) {
	case map[string]any:
		if len(v) == 0 {
			out.WriteString("{}")
			return nil
		}
		var keys []string
		seen := map[string]bool{}
		for _, key := range order[path] {
			if _, present := v[key]; present && !seen[key] {
				keys = append(keys, key)
				seen[key] = true
			}
		}
		var added []string
		for key := range v {
			if !seen[key] {
				added = append(added, key)
			}
		}
		sort.Strings(added)
		keys = append(keys, added...)
		out.WriteString("{\n")
		for i, key := range keys {
			out.WriteString(indent + "  ")
			if err := encodeScalar(out, key); err != nil {
				return err
			}
			out.WriteString(": ")
			if err := encodeOrdered(out, v[key], path+"/"+key, order, indent+"  "); err != nil {
				return err
			}
			if i < len(keys)-1 {
				out.WriteString(",")
			}
			out.WriteString("\n")
		}
		out.WriteString(indent + "}")
	case []any:
		if len(v) == 0 {
			out.WriteString("[]")
			return nil
		}
		out.WriteString("[\n")
		for i, item := range v {
			out.WriteString(indent + "  ")
			if err := encodeOrdered(out, item, path+"/"+strconv.Itoa(i), order, indent+"  "); err != nil {
				return err
			}
			if i < len(v)-1 {
				out.WriteString(",")
			}
			out.WriteString("\n")
		}
		out.WriteString(indent + "]")
	default:
		return encodeScalar(out, v)
	}
	return nil
}

func encodeScalar(out *bytes.Buffer, value any) error {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode %v: %w", value, err)
	}
	out.WriteString(strings.TrimSuffix(buffer.String(), "\n"))
	return nil
}
