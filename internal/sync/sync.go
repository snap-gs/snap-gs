package sync

type Meta struct {
	ContentType        string            `json:"content_type,omitempty"`
	ContentDisposition string            `json:"content_disposition,omitempty"`
	ContentLanguage    string            `json:"content_language,omitempty"`
	ContentEncoding    string            `json:"content_encoding,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}
