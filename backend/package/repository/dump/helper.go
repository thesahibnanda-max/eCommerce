package dumpRepository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	redactedValue = "[redacted]"
	keyColumn     = "id"
	base64Field   = "base64"
)

var identifierPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func encodeKey(key []byte) string {
	if utf8.Valid(key) {
		return string(key)
	}
	return base64Field + ":" + base64.StdEncoding.EncodeToString(key)
}

func encodeBoltValue(value []byte, redact []string) (string, error) {
	if !utf8.Valid(value) {
		return utils.FromObjectToStringJSON(map[string]string{base64Field: base64.StdEncoding.EncodeToString(value)})
	}
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		if parsed, err := utils.FromStringToObjectJSON[any](string(trimmed)); err == nil {
			object, isObject := parsed.(map[string]any)
			if !isObject || !redactKeys(object, redact) {
				return string(trimmed), nil
			}
			return utils.FromObjectToStringJSON(object)
		}
	}
	return utils.FromObjectToStringJSON(string(value))
}

func redactKeys(object map[string]any, redact []string) bool {
	changed := false
	for _, key := range redact {
		if _, ok := object[key]; ok {
			object[key] = redactedValue
			changed = true
		}
	}
	return changed
}

func dumpTable(ctx context.Context, db *sql.DB, spec TableSpec) (Table, error) {
	rows, err := db.QueryContext(ctx, "SELECT * FROM "+spec.Name+" ORDER BY "+keyColumn)
	if err != nil {
		return Table{}, fmt.Errorf("%w: table %q: %w", ErrReadFailed, spec.Name, err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return Table{}, fmt.Errorf("%w: table %q: %w", ErrReadFailed, spec.Name, err)
	}
	keyIndex := slices.Index(columns, keyColumn)
	if keyIndex < 0 {
		return Table{}, fmt.Errorf("%w: %q", ErrKeyMissing, spec.Name)
	}
	table := Table{Name: spec.Name}
	values := make([]any, len(columns))
	targets := make([]any, len(columns))
	for i := range values {
		targets[i] = &values[i]
	}
	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return Table{}, fmt.Errorf("%w: table %q: %w", ErrReadFailed, spec.Name, err)
		}
		record := make(map[string]any, len(columns))
		for i, column := range columns {
			if slices.Contains(spec.Skip, column) {
				continue
			}
			record[column] = sqlValue(values[i])
		}
		raw, err := utils.FromObjectToStringJSON(record)
		if err != nil {
			return Table{}, fmt.Errorf("%w: table %q: %w", ErrEncodingFailed, spec.Name, err)
		}
		table.Rows = append(table.Rows, Record{Key: keyString(values[keyIndex]), JSON: raw})
	}
	if err := rows.Err(); err != nil {
		return Table{}, fmt.Errorf("%w: table %q: %w", ErrReadFailed, spec.Name, err)
	}
	return table, nil
}

func sqlValue(value any) any {
	switch v := value.(type) {
	case []byte:
		if utf8.Valid(v) {
			return string(v)
		}
		return map[string]string{base64Field: base64.StdEncoding.EncodeToString(v)}
	case time.Time:
		return v.Format(time.RFC3339Nano)
	default:
		return v
	}
}

func keyString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return encodeKey(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return fmt.Sprint(v)
	}
}

func recoverAsError(err *error) {
	if recovered := recover(); recovered != nil {
		*err = fmt.Errorf("%w: %v", ErrPanicked, recovered)
	}
}
