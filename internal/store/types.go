package store

// Session is one transcript file's summary, as stored in the index.
type Session struct {
	SessionID       string
	FilePath        string
	ProjectPath     string // cwd from inside the transcript
	ProjectDir      string // the projects/<encoded> folder
	DisplayName     string
	CustomTitle     string
	AITitle         string
	FirstPrompt     string
	MessageCount    int
	FileSize        int64
	LastActivity    string // ISO-8601
	FirstParentUUID string
	FileMtime       int64 // unix seconds
	IndexedAt       string
}

// Message is one text-bearing message row, used both for the FTS index and
// the resume/branch chain.
type Message struct {
	SessionID  string
	UUID       string
	ParentUUID string
	Role       string
	Timestamp  string
	Seq        int
	Body       string // full text, indexed in FTS5 (not stored verbatim in messages)
}

// SearchHit is one full-text search result.
type SearchHit struct {
	SessionID   string
	DisplayName string
	ProjectPath string
	Snippet     string // FTS5 snippet() with <mark> highlights
}
