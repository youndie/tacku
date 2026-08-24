package httpsrv_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/youndie/tacku/server/internal/docsboard"
	"github.com/youndie/tacku/server/internal/httpsrv"
	"github.com/youndie/tacku/server/internal/render"
)

// The repository this reads is configuration and never a name in this repository, so the fixture is
// a repository of the same shape and no relation: an index with a stage table and two items.
const fixtureIndex = `# The backlog of a lending system

| Идентификатор | Этап |
|---|---|
| ` + "`stage-one`" + ` | Getting the loans right |
`

func fixtureArchive(t *testing.T) []byte {
	t.Helper()

	files := map[string]string{
		"backlog.md": fixtureIndex,
		"backlog/B-02-overdue-notices.md": "---\nid: B-02\ntitle: \"A failed overdue notice is recorded as sent\"\n" +
			"status: open\npriority: P1\nsize: S\nstage: stage-one\n---\n\n# B-02\n\n" +
			"The loans service records a notice as sent before the mail gateway has accepted it, so\n" +
			"a notice that was never delivered is indistinguishable from one that was.\n\n" +
			"- the sending is retried while the gateway refuses\n",
		"backlog/B-01-hold-positions.md": "---\nid: B-01\ntitle: \"Queue positions are recomputed on every read\"\n" +
			"status: done\npriority: P2\nsize: M\nstage: stage-one\n---\n",
	}

	var buffer bytes.Buffer
	zipped := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(zipped)
	for name, body := range files {
		if err := archive.WriteHeader(&tar.Header{Name: "example-docs-abc1234/" + name, Mode: 0o644, Size: int64(len(body))}); err != nil {
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

// sourceStand answers as the forge does, and can be told to refuse.
func sourceStand(t *testing.T) (*httptest.Server, *int) {
	t.Helper()

	archive := fixtureArchive(t)
	refuse := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if refuse != 0 {
			// A refusal that quotes the request, which is the shape that carries a credential back
			// out into a message somebody pastes into a chat.
			http.Error(w, "no: "+r.Header.Get("Authorization"), refuse)
			return
		}
		if strings.Contains(r.URL.Path, "/commits/") {
			w.Write([]byte("abc1234"))
			return
		}
		w.Write(archive)
	}))
	t.Cleanup(server.Close)
	return server, &refuse
}

func withSource(t *testing.T, server *httptest.Server, ttl time.Duration) func(*httpsrv.Config) {
	return func(config *httpsrv.Config) {
		config.DocsBoard = docsboard.New(docsboard.Config{
			Repo: "example/docs", Ref: "main", Root: "backlog", Index: "backlog.md",
			Token: "a-secret-value", TTL: ttl, API: server.URL, Client: server.Client(),
		})
	}
}

func TestWithoutASourceTheViewDoesNotExist(t *testing.T) {
	r := newResource(t)
	token := r.reader(t)

	response, _ := r.get(t, render.DocsBoardPath, token, "")
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("экран отдан кодом %d, хотя источник не задан", response.StatusCode)
	}

	_, body := r.get(t, "/graph", token, "")
	if strings.Contains(string(body), render.LinkDocsBoard) {
		t.Error("граф называет экран, которого этот контур не обслуживает — это кнопка в никуда")
	}
}

func TestWithASourceTheViewIsReachable(t *testing.T) {
	stand, _ := sourceStand(t)
	r := newResourceWith(t, withSource(t, stand, time.Hour))
	token := r.reader(t)

	var graph struct {
		Routes []render.Route `json:"routes"`
	}
	_, body := r.get(t, "/graph", token, "")
	if err := json.Unmarshal(body, &graph); err != nil {
		t.Fatal(err)
	}
	var carried *render.Route
	for i, route := range graph.Routes {
		if route.Deeplink == render.LinkDocsBoard {
			carried = &graph.Routes[i]
		}
	}
	if carried == nil {
		t.Fatal("источник задан, а маршрута в графе нет")
	}
	if carried.Rail == "" {
		t.Error("маршрут есть, но в панель навигации не попадает — попасть на него неоткуда")
	}

	response, screen := r.get(t, carried.Endpoint, token, "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("экран ответил %d", response.StatusCode)
	}
	for _, expected := range []string{
		"The backlog of a lending system", // заголовок из индекса источника
		"stage-one",                       // колонка названа идентификатором этапа
		"A failed overdue notice",         // открытая задача
		"1 open task · 1 done",            // сводка
	} {
		if !strings.Contains(string(screen), expected) {
			t.Errorf("на экране нет %q", expected)
		}
	}
	if strings.Contains(string(screen), "Queue positions") {
		t.Error("сделанная задача нарисована карточкой, а должна быть посчитана")
	}
}

// The two states a board must never render identically.
func TestAnUnreachableSourceSaysSoOverTheLastReading(t *testing.T) {
	stand, refuse := sourceStand(t)
	r := newResourceWith(t, withSource(t, stand, time.Nanosecond))
	token := r.reader(t)

	if _, screen := r.get(t, render.DocsBoardPath, token, ""); !strings.Contains(string(screen), "A failed overdue notice") {
		t.Fatal("первое чтение не удалось — дальше проверять нечего")
	}

	*refuse = http.StatusNotFound
	response, screen := r.get(t, render.DocsBoardPath, token, "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("экран ответил %d, а прошлый снимок был в руках", response.StatusCode)
	}
	if !strings.Contains(string(screen), "(404)") {
		t.Error("источник отказал, а экран не назвал чем именно — дальше только гадать")
	}
	if !strings.Contains(string(screen), "A failed overdue notice") {
		t.Error("прошлое чтение выброшено")
	}
}

func TestASourceThatWasNeverReadIsAScreenAndNotAnError(t *testing.T) {
	stand, refuse := sourceStand(t)
	*refuse = http.StatusUnauthorized
	r := newResourceWith(t, withSource(t, stand, time.Hour))
	token := r.reader(t)

	response, screen := r.get(t, render.DocsBoardPath, token, "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("экран ответил %d: клиент покажет свою ошибку вместо объяснения", response.StatusCode)
	}
	if !strings.Contains(string(screen), "could not be read") {
		t.Error("экран не сказал, что прочитать не удалось")
	}
	// Первая выкатка встретила именно этот экран, и на нём не было ничего, по чему можно действовать:
	// сервер знал, чем ответил источник, и оставил это при себе.
	if !strings.Contains(string(screen), "(401)") {
		t.Error("экран не назвал отказ: 401 и 404 чинятся по-разному, а выглядят одинаково")
	}
	if strings.Contains(string(screen), "a-secret-value") {
		t.Error("на экране оказались учётные данные")
	}
}

func TestTheViewOffersNothingThatWrites(t *testing.T) {
	stand, _ := sourceStand(t)
	r := newResourceWith(t, withSource(t, stand, time.Hour))

	_, screen := r.get(t, render.DocsBoardPath, r.reader(t), "")
	for _, forbidden := range []string{`"perform"`, `"submit_form"`} {
		if strings.Contains(string(screen), forbidden) {
			t.Errorf("витрина несёт действие %s — она обязана быть окном, а не доской", forbidden)
		}
	}
}

// A card that names an item and cannot open it is a dead end, and the board was a wall of them: the
// vocabulary has no action that leaves the application, so this screen is the only place the item's
// own text can be read.
func TestAnItemCanBeRead(t *testing.T) {
	stand, _ := sourceStand(t)
	r := newResourceWith(t, withSource(t, stand, time.Hour))
	token := r.reader(t)

	_, board := r.get(t, render.DocsBoardPath, token, "")
	if !strings.Contains(string(board), render.LinkDocsItem+"B-02") {
		t.Fatal("карточка никуда не ведёт")
	}

	response, screen := r.get(t, render.DocsItemPath+"B-02", token, "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("экран задачи ответил %d", response.StatusCode)
	}
	for _, expected := range []string{
		"A failed overdue notice",           // заголовок
		"B-02 · P1 · S",                     // та же метастрока, что на карточке
		"a notice that was never delivered", // текст самой задачи, из файла
		"· the sending is retried",          // список, маркер выставлен нами
	} {
		if !strings.Contains(string(screen), expected) {
			t.Errorf("на экране нет %q", expected)
		}
	}
}

func TestAnItemNobodyReadIsNotAScreen(t *testing.T) {
	stand, _ := sourceStand(t)
	r := newResourceWith(t, withSource(t, stand, time.Hour))

	response, _ := r.get(t, render.DocsItemPath+"B-999", r.reader(t), "")
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("выдуманный идентификатор ответил %d, а не 404", response.StatusCode)
	}
}
