package transcript

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   Event
		wantOK bool
	}{
		{
			name:   "user message",
			line:   `{"type":"user","timestamp":"2026-10-08T13:58:09Z","sessionId":"s1"}`,
			want:   Event{Time: time.Date(2026, 10, 8, 13, 58, 9, 0, time.UTC), SessionID: "s1"},
			wantOK: true,
		},
		{
			name:   "assistant message",
			line:   `{"type":"assistant","timestamp":"2026-10-08T13:59:00Z","sessionId":"s1"}`,
			want:   Event{Time: time.Date(2026, 10, 8, 13, 59, 0, 0, time.UTC), SessionID: "s1"},
			wantOK: true,
		},
		{
			name: "other event type",
			line: `{"type":"attachment","timestamp":"2026-10-08T13:58:10Z","sessionId":"s1"}`,
		},
		{
			name: "no timestamp",
			line: `{"type":"user","sessionId":"s1"}`,
		},
		{
			name: "not JSON",
			line: `not json`,
		},
		{
			name: "cut off",
			line: `{"type":"user","timestamp":"2026-10-08T1`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseLine([]byte(tt.line))
			if ok != tt.wantOK {
				t.Fatalf("parseLine() ok = %v, want %v", ok, tt.wantOK)
			}
			if !got.Time.Equal(tt.want.Time) || got.SessionID != tt.want.SessionID {
				t.Errorf("parseLine() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// The fixture also holds lines that must be skipped: a non-message event, an
// attachment, a line that isn't JSON, a message without a timestamp, and a
// last line cut off without a newline.
func TestRead(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	got, err := Read(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	want := []Event{
		{Time: time.Date(2026, 10, 8, 13, 58, 9, 783_000_000, time.UTC), SessionID: "s1"},
		{Time: time.Date(2026, 10, 8, 13, 59, 0, 0, time.UTC), SessionID: "s1"},
		{Time: time.Date(2026, 10, 8, 14, 5, 0, 0, time.UTC), SessionID: "s1"},
	}
	if len(got) != len(want) {
		t.Fatalf("Read() returned %d events, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if !got[i].Time.Equal(want[i].Time) || got[i].SessionID != want[i].SessionID {
			t.Errorf("event %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// Real transcripts have lines over 1 MB, beyond bufio.Scanner's default limit.
func TestReadLongLine(t *testing.T) {
	const textSize = 2 << 20 // 2 MiB
	text := strings.Repeat("x", textSize)
	in := `{"type":"user","timestamp":"2026-10-08T13:58:09Z","sessionId":"s1","message":{"content":"` +
		text + `"}}` + "\n"

	got, err := Read(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Read() returned %d events, want 1", len(got))
	}
}

var errDisk = errors.New("disk error")

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestReadError(t *testing.T) {
	_, err := Read(failingReader{err: errDisk})
	if !errors.Is(err, errDisk) {
		t.Errorf("Read() error = %v, want it to wrap %v", err, errDisk)
	}
}

// A complete last line without a trailing newline still counts; only lines
// that don't decode are skipped.
func TestReadLastLineWithoutNewline(t *testing.T) {
	in := `{"type":"user","timestamp":"2026-10-08T13:58:09Z","sessionId":"s1"}`

	got, err := Read(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Read() returned %d events, want 1", len(got))
	}
}

func userLine(timestamp, session string) string {
	return `{"type":"user","timestamp":"` + timestamp + `","sessionId":"` + session + `"}` + "\n"
}

const s1Path = "project-a/s1.jsonl"

// oneTranscript returns a tree with one transcript, at s1Path.
func oneTranscript() fstest.MapFS {
	return fstest.MapFS{s1Path: {Data: []byte(userLine("2026-10-08T10:00:00Z", "s1"))}}
}

func TestReadFS(t *testing.T) {
	fsys := fstest.MapFS{
		"project-b/s2.jsonl":                 {Data: []byte(userLine("2026-10-08T11:00:00Z", "s2"))},
		s1Path:                               {Data: []byte(userLine("2026-10-08T10:00:00Z", "s1"))},
		"project-a/sessions-index.json":      {Data: []byte(userLine("2026-10-08T12:00:00Z", "not-a-transcript"))},
		"project-a/s1/subagents/agent.jsonl": {Data: []byte(userLine("2026-10-08T13:00:00Z", "subagent"))},
		"top-level.jsonl":                    {Data: []byte(userLine("2026-10-08T14:00:00Z", "top-level"))},
	}

	got, err := ReadFS(fsys)
	if err != nil {
		t.Fatal(err)
	}
	sessions := make([]string, 0, len(got))
	for _, e := range got {
		sessions = append(sessions, e.SessionID)
	}
	// Lexical order of the paths: project-a before project-b.
	want := []string{"s1", "s2"}
	if !slices.Equal(sessions, want) {
		t.Errorf("ReadFS() sessions = %v, want %v", sessions, want)
	}
}

var errOpen = errors.New("open failed")

// openErrorFS lists files like the embedded MapFS but fails to open them.
type openErrorFS struct{ fstest.MapFS }

func (openErrorFS) Open(string) (fs.File, error) { return nil, errOpen }

func TestReadFSOpenError(t *testing.T) {
	fsys := openErrorFS{oneTranscript()}

	_, err := ReadFS(fsys)
	if !errors.Is(err, errOpen) {
		t.Errorf("ReadFS() error = %v, want it to wrap %v", err, errOpen)
	}
}

var errClose = errors.New("close failed")

// brokenFile returns readErr from Read and closeErr from Close; nil readErr
// means an empty file.
type brokenFile struct {
	fs.File

	readErr, closeErr error
}

func (f brokenFile) Read([]byte) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	return 0, io.EOF
}

func (f brokenFile) Close() error { return f.closeErr }

// brokenFileFS lists files like the embedded MapFS but opens each as file.
type brokenFileFS struct {
	fstest.MapFS

	file brokenFile
}

func (b brokenFileFS) Open(string) (fs.File, error) { return b.file, nil }

func TestReadFSReadError(t *testing.T) {
	fsys := brokenFileFS{oneTranscript(), brokenFile{readErr: errDisk}}

	_, err := ReadFS(fsys)
	if !errors.Is(err, errDisk) {
		t.Fatalf("ReadFS() error = %v, want it to wrap %v", err, errDisk)
	}
	if !strings.Contains(err.Error(), s1Path) {
		t.Errorf("ReadFS() error = %v, want it to name %s", err, s1Path)
	}
}

func TestReadFSCloseError(t *testing.T) {
	fsys := brokenFileFS{oneTranscript(), brokenFile{closeErr: errClose}}

	_, err := ReadFS(fsys)
	if !errors.Is(err, errClose) {
		t.Fatalf("ReadFS() error = %v, want it to wrap %v", err, errClose)
	}
	if !strings.Contains(err.Error(), s1Path) {
		t.Errorf("ReadFS() error = %v, want it to name %s", err, s1Path)
	}
}

func TestParseLinePaths(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{
			name: "file tools and a Bash command",
			line: `{"type":"assistant","timestamp":"2026-10-08T10:00:00Z","sessionId":"s1","message":{"content":[` +
				`{"type":"text","text":"see /not/a/tool/use"},` +
				`{"type":"tool_use","name":"Read","input":{"file_path":"/repo/a.go"}},` +
				`{"type":"tool_use","name":"NotebookEdit","input":{"notebook_path":"/repo/n.ipynb"}},` +
				`{"type":"tool_use","name":"Grep","input":{"path":"/repo/internal","pattern":"x"}},` +
				`{"type":"tool_use","name":"Bash","input":{"command":"cd ~/code/oss && go test ./..."}}]}}`,
			want: []string{"/repo/a.go", "/repo/n.ipynb", "/repo/internal", "~/code/oss"},
		},
		{
			name: "a relative path is not a file system path",
			line: `{"type":"assistant","timestamp":"2026-10-08T10:00:00Z","sessionId":"s1","message":{"content":[` +
				`{"type":"tool_use","name":"Artifact","input":{"path":"index.html"}}]}}`,
		},
		{
			name: "a plain user message",
			line: `{"type":"user","timestamp":"2026-10-08T10:00:00Z","sessionId":"s1","message":{"content":"open /etc/hosts"}}`,
		},
		{
			name: "tool results are not tool uses",
			line: `{"type":"user","timestamp":"2026-10-08T10:00:00Z","sessionId":"s1","message":{"content":[` +
				`{"type":"tool_result","content":"/repo/a.go"}]}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseLine([]byte(tt.line))
			if !ok {
				t.Fatal("parseLine() ok = false, want true")
			}
			if !slices.Equal(got.Paths, tt.want) {
				t.Errorf("Paths = %q, want %q", got.Paths, tt.want)
			}
		})
	}
}

func TestCommandPaths(t *testing.T) {
	tests := []struct {
		command string
		want    []string
	}{
		{command: "cd ~/code/oss && git status", want: []string{"~/code/oss"}},
		{command: "go test ./..."},
		{command: "cat /a/b.go|grep x", want: []string{"/a/b.go"}},
		{command: "tool --dir=/tmp/x", want: []string{"/tmp/x"}},
		{command: `ls "/x/y z"`, want: []string{"/x/y"}},
		{command: "echo hi>~/out.txt; cd ~", want: []string{"~/out.txt", "~"}},
		{command: "curl https://example.com/a"},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			if got := commandPaths(tt.command); !slices.Equal(got, tt.want) {
				t.Errorf("commandPaths(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}
