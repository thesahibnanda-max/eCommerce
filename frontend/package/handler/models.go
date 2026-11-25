package frontendHandler

type asset struct {
	contentType string
	body        []byte
	gzipped     []byte
	etag        string
	cache       string
}
