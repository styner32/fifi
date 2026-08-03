package auth

// Rows extracts a slice of maps from RESTResponse Body by key.
// If the value is a map, it wraps it in a single-element slice.
func (r *RESTResponse) Rows(key string) []map[string]any {
	if r == nil || r.Body == nil {
		return nil
	}
	switch typed := r.Body[key].(type) {
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if row, ok := item.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{typed}
	default:
		return nil
	}
}

// FirstRow returns the first row from the first key that contains data.
func (r *RESTResponse) FirstRow(keys ...string) map[string]any {
	for _, key := range keys {
		if rows := r.Rows(key); len(rows) > 0 {
			return rows[0]
		}
	}
	return nil
}
