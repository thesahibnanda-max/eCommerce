package dumpRepository

type Table struct {
	Name string
	Rows []Record
}

type Record struct {
	Key  string
	JSON string
}

type BoltOptions struct {
	Skip   []string
	Redact []string
}

type TableSpec struct {
	Name string
	Skip []string
}
