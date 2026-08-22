package httpsrv_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/youndie/tacku/server/internal/domain"
)

// Nobody sees anybody else's channel.
//
// This is the one requirement of a live channel that the conformance kit says outright it cannot
// check: it needs two live identities and the kit's transport is request/response. A frame from
// somebody else's topic is a leak, and a leak nobody checks for is one every implementation is free
// to ship — so the absence of a portable check is the reason to write a local one, not a reason to
// have none.
func TestNobodySeesAnotherPersonsChannel(t *testing.T) {
	r := newResource(t)
	r.fill(t, 2)

	mine := r.reader(t)
	theirs := r.readerAs(t, "ivan", "ivan@tacku.team", "Ivan Sokolov")

	// Something happens, and it happens on a board both of them can read. What separates them is
	// the topic, not visibility of the data.
	moved := r.move(t, "TAC-1", domain.StatusInProgress)

	frames := r.listen(t, mine, 2*time.Second)
	if len(frames) == 0 {
		t.Fatal("the channel of the person who is watching carried nothing, so this check had nothing to compare")
	}
	if !strings.Contains(strings.Join(frames, "\n"), string(moved)) {
		t.Errorf("the watcher's channel never mentioned %s, which is what changed: %v", moved, frames)
	}

	// The other person has read up to the end already, so their channel is quiet — and it stays
	// quiet even while somebody else's is not. The boundary is moved through the store rather than
	// through the screen, because what is being separated here is the topic, not the interface.
	latest, err := r.store.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := r.store.MarkSeen(context.Background(), "ivan", latest); err != nil {
		t.Fatal(err)
	}
	quiet := r.listen(t, theirs, time.Second)
	if len(quiet) != 0 {
		t.Errorf("a person who had read everything received %d frames of somebody else's activity: %v",
			len(quiet), quiet)
	}
}

// listen holds the channel open for a while and returns the data lines it carried.
func (r *resource) listen(t *testing.T, token string, hold time.Duration) []string {
	t.Helper()

	ctx, stop := context.WithTimeout(context.Background(), hold)
	defer stop()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url+"/updates", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()

	if kind := response.Header.Get("Content-Type"); kind != "text/event-stream" {
		t.Fatalf("the channel answered %q, not an event stream", kind)
	}

	found := []string{}
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "data: ") {
			found = append(found, strings.TrimPrefix(line, "data: "))
		}
	}
	return found
}

// move changes a status the way the board does and answers which task it was.
func (r *resource) move(t *testing.T, id domain.TaskID, to domain.Status) domain.TaskID {
	t.Helper()

	if _, err := r.store.MoveTask(context.Background(), id, to, domain.Human("anna"), domain.SurfaceBoard); err != nil {
		t.Fatal(err)
	}
	return id
}

// A pushed card still works when it lands.
//
// A frame replaces a node on a screen the server is not looking at, so everything that node needs
// has to travel with it. The first version of this frame carried a button whose address was the
// empty string: the card looked right, and pressing it posted into nothing — the same silence as a
// form submitting to an address that does not exist, and found the same way, by reading what went
// over the wire rather than by looking at the picture.
func TestAPushedCardCarriesEverythingItNeeds(t *testing.T) {
	r := newResource(t)
	r.fill(t, 2)
	token := r.reader(t)
	r.move(t, "TAC-2", domain.StatusInProgress)

	frames := r.listen(t, token, 2*time.Second)
	if len(frames) == 0 {
		t.Fatal("nothing was pushed, so this check had nothing to look at")
	}

	for _, frame := range frames {
		var message struct {
			ComponentID string          `json:"componentId"`
			Component   json.RawMessage `json:"component"`
		}
		if err := json.Unmarshal([]byte(frame), &message); err != nil {
			t.Fatalf("a frame did not decode: %v", err)
		}
		if message.ComponentID == "" {
			t.Error("a frame names no component, so nothing on the screen could be replaced by it")
		}
		if strings.Contains(string(message.Component), `"url":""`) {
			t.Errorf("%s carries a control with no address: %s", message.ComponentID, message.Component)
		}
	}
}

// head asks for a route and reads only its headers, which is all there is to know about an endpoint
// that never stops answering.
func (r *resource) head(t *testing.T, path, token string) int {
	t.Helper()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	_ = response.Body.Close()
	return response.StatusCode
}
