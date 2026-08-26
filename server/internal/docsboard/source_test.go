package docsboard

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// archiveOf builds what the source hands out: a repository archive, every path under a directory
// named after the commit.
func archiveOf(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	zipped := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(zipped)
	for name, body := range files {
		header := &tar.Header{Name: "example-docs-abc1234/" + name, Mode: 0o644, Size: int64(len(body))}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipped.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

type stand struct {
	server   *httptest.Server
	head     atomic.Value // string
	archive  atomic.Value // []byte
	heads    atomic.Int64
	archives atomic.Int64
	auth     atomic.Value // string
	down     atomic.Bool
}

func newStand(t *testing.T, files map[string]string) *stand {
	t.Helper()

	s := &stand{}
	s.head.Store("abc1234")
	s.archive.Store(archiveOf(t, files))

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.auth.Store(r.Header.Get("Authorization"))
		if s.down.Load() {
			// A refusal quoting the request, which is exactly the shape that carries a credential
			// back out into a message somebody pastes into a chat.
			http.Error(w, "no: "+r.Header.Get("Authorization"), http.StatusUnauthorized)
			return
		}
		switch {
		case strings.Contains(r.URL.Path, "/commits/"):
			s.heads.Add(1)
			w.Write([]byte(s.head.Load().(string)))
		case strings.Contains(r.URL.Path, "/tarball/"):
			s.archives.Add(1)
			w.Write(s.archive.Load().([]byte))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.server.Close)
	return s
}

const indexFile = `# The backlog of a lending system

| Идентификатор | Этап |
|---|---|
| ` + "`stage-one`" + ` | First |
`

var files = map[string]string{
	"backlog.md":                  indexFile,
	"backlog/B-01-first.md":       frontmatter("B-01", "open", "P1", "stage-one"),
	"backlog/B-02-second.md":      frontmatter("B-02", "done", "P2", "stage-one"),
	"backlog/README.md":           "not an item\n",
	"backlog/../templates/x.md":   "not in the tree\n",
	"features/feature-lending.md": "# a document that is not a backlog item\n",
}

func sourceOn(s *stand, ttl time.Duration) *Source {
	return New(Config{
		Key: "example", Repo: "example/docs", Ref: "main", Root: "backlog", Index: "backlog.md",
		Private: true, Token: "a-secret-value", TTL: ttl, API: s.server.URL, Client: s.server.Client(),
	})
}

func TestASnapshotIsWhatTheRepositoryHolds(t *testing.T) {
	stand := newStand(t, files)
	snapshot, err := sourceOn(stand, time.Hour).Load(context.Background())
	if err != nil {
		t.Fatalf("не прочиталось: %v", err)
	}

	if len(snapshot.Items) != 2 {
		t.Fatalf("задач %d, а в дереве две (README задачей не является)", len(snapshot.Items))
	}
	if snapshot.Title != "The backlog of a lending system" {
		t.Errorf("заголовок %q взят не из индекса", snapshot.Title)
	}
	if len(snapshot.Stages) != 1 || snapshot.Stages[0] != "stage-one" {
		t.Errorf("этапы %v", snapshot.Stages)
	}
	if snapshot.Head != "abc1234" {
		t.Errorf("снимок не назвал коммит: %q", snapshot.Head)
	}
}

func TestTheArchiveIsFetchedOnlyWhenTheSourceMoved(t *testing.T) {
	stand := newStand(t, files)
	source := sourceOn(stand, time.Nanosecond)

	for range 3 {
		if _, err := source.Load(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := stand.archives.Load(); got != 1 {
		t.Errorf("архив скачан %d раз(а) на три чтения — коммит не сверялся", got)
	}
	if got := stand.heads.Load(); got != 3 {
		t.Errorf("коммит спрошен %d раз(а) из трёх", got)
	}

	stand.head.Store("def5678")
	if _, err := source.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := stand.archives.Load(); got != 2 {
		t.Errorf("источник сдвинулся, а архив скачан %d раз(а)", got)
	}
}

func TestAFreshSnapshotIsNotRefetched(t *testing.T) {
	stand := newStand(t, files)
	source := sourceOn(stand, time.Hour)

	for range 3 {
		if _, err := source.Load(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := stand.heads.Load(); got != 1 {
		t.Errorf("в пределах срока годности источник спрошен %d раз(а)", got)
	}
}

// The two states a board must never render identically: nothing is open, and nothing has been read.
func TestAFailedRefreshKeepsTheLastReadingAndSaysSo(t *testing.T) {
	stand := newStand(t, files)
	source := sourceOn(stand, time.Nanosecond)

	if _, err := source.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	stand.down.Store(true)

	snapshot, err := source.Load(context.Background())
	if err == nil {
		t.Fatal("источник лежит, а чтение прошло молча")
	}
	if len(snapshot.Items) != 2 {
		t.Errorf("прошлый снимок не сохранён: задач %d", len(snapshot.Items))
	}
	if strings.Contains(err.Error(), "a-secret-value") {
		t.Errorf("отказ вынес наружу учётные данные: %v", err)
	}
}

// A typo in either of the two settings that decide what is read produces exactly the picture of a
// backlog where everything is finished.
func TestAnEmptyTreeIsARefusalAndNotAnEmptyBoard(t *testing.T) {
	stand := newStand(t, map[string]string{"backlog.md": indexFile})
	if _, err := sourceOn(stand, time.Hour).Load(context.Background()); err == nil {
		t.Fatal("дерево без задач прочиталось как пустая доска")
	}
}

func TestTheCredentialIsCarried(t *testing.T) {
	stand := newStand(t, files)
	if _, err := sourceOn(stand, time.Hour).Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, _ := stand.auth.Load().(string); got != "Bearer a-secret-value" {
		t.Errorf("заголовок авторизации %q — закрытый источник так не откроется", got)
	}
}

func TestWithoutARepositoryThereIsNoSource(t *testing.T) {
	if (Config{Ref: "main", Token: "t"}).Configured() {
		t.Fatal("источник считается заданным без репозитория")
	}
}

// A line when the outcome changes, and not one per visit: the board is read on every visit, and a
// log with a line each is one nobody greps twice.
func TestOnlyAChangeOfOutcomeIsWrittenDown(t *testing.T) {
	stand := newStand(t, files)

	var lines []string
	config := sourceOn(stand, time.Nanosecond).Config()
	config.Log = func(format string, args ...any) {
		lines = append(lines, fmt.Sprintf(format, args...))
	}
	source := New(config)

	for range 3 {
		source.Load(context.Background())
	}
	if len(lines) != 1 {
		t.Fatalf("три удачных чтения дали %d строк: %v", len(lines), lines)
	}

	stand.down.Store(true)
	for range 3 {
		source.Load(context.Background())
	}
	if len(lines) != 2 {
		t.Fatalf("падение и два повтора дали %d строк всего: %v", len(lines), lines)
	}

	// Координаты в строке: первый вопрос под «не прочиталось» — какой репозиторий процесс вообще
	// пытается читать, а это вопрос к его собственной конфигурации.
	if !strings.Contains(lines[1], "example/docs") || !strings.Contains(lines[1], "backlog") {
		t.Errorf("строка не назвала, что читалось: %q", lines[1])
	}
	if strings.Contains(lines[1], "a-secret-value") {
		t.Errorf("учётные данные попали в лог: %q", lines[1])
	}

	stand.down.Store(false)
	source.Load(context.Background())
	if len(lines) != 3 {
		t.Errorf("восстановление не записано: %v", lines)
	}
}
