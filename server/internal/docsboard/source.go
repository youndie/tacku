package docsboard

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

// Config says where the backlog being looked at lives.
//
// Every field arrives from the environment of the running process, and none of them has a default
// naming anything. This repository is public and stands next to closed ones; a repository name
// committed here cannot be taken back out of the clones. The gate keeps fingerprints of the names
// that must not appear (scripts/no_private_names.py), but the reason the gate has nothing to catch
// is that the source is configuration and not code.
type Config struct {
	// Repo is `owner/name`.
	Repo string
	// Ref is a branch, tag or commit; empty means the repository's default branch.
	Ref string
	// Root is the directory holding the item files, relative to the repository root. The method
	// puts them under `docs/backlog`; a repository whose whole content is documentation puts them
	// at `backlog`, which is why this is a setting and not a constant.
	Root string
	// Index is the file carrying the stage table and the title, relative to the repository root.
	Index string
	// Token is a read-only credential for the source repository.
	Token string
	// TTL is how long a snapshot is served without asking the source whether it moved.
	TTL time.Duration

	// API is the base of the GitHub API. A field so that a test can answer for it; there is no
	// second production value.
	API string

	Client *http.Client
}

// Configured reports whether there is a source at all. When there is not, the screen does not
// exist: no route, no graph entry, no button leading nowhere.
func (c Config) Configured() bool { return c.Repo != "" }

func (c Config) withDefaults() Config {
	if c.Ref == "" {
		c.Ref = "main"
	}
	if c.Root == "" {
		c.Root = "docs/backlog"
	}
	if c.Index == "" {
		c.Index = "backlog.md"
	}
	if c.TTL == 0 {
		c.TTL = time.Minute
	}
	if c.API == "" {
		c.API = "https://api.github.com"
	}
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 30 * time.Second}
	}
	return c
}

// Snapshot is one reading of the source.
type Snapshot struct {
	// Title is what the source calls its own backlog: the first heading of the index.
	Title  string
	Stages []string
	Items  []Item

	// Head is the commit this was read at, and TakenAt is when. Both are shown: a board with no
	// date cannot tell a person that it stopped being refreshed an hour ago.
	Head    string
	TakenAt time.Time
}

// Empty reports a snapshot that has never been filled.
func (s Snapshot) Empty() bool { return s.Head == "" }

// Source keeps the last reading and refreshes it when the source has moved.
//
// Refreshing is two requests and usually one: the commit of a ref costs a few bytes, and the
// archive is fetched only when that commit differs from the one in hand. A poll that downloaded
// the tree every minute would work just as well and be invisible until somebody looked at the rate
// limit.
type Source struct {
	config Config

	mu       sync.Mutex
	snapshot Snapshot
	checked  time.Time
}

func New(config Config) *Source { return &Source{config: config.withDefaults()} }

// Config gives back the settings in use, defaults filled in.
func (s *Source) Config() Config { return s.config }

// FileURL is where a person goes to read the item itself.
func (s *Source) FileURL(item Item) string {
	return fmt.Sprintf("https://github.com/%s/blob/%s/%s", s.config.Repo, s.config.Ref, item.Path)
}

// Load returns what the board should draw.
//
// A failure to refresh does not empty the board: the last good snapshot comes back **together with
// the error**, and the screen says both. The two states a person must be able to tell apart are "the
// repository has nothing open" and "this has not been read since morning", and an empty board
// renders them identically. Only a failure with nothing in hand is a plain error.
func (s *Source) Load(ctx context.Context) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.snapshot.Empty() && time.Since(s.checked) < s.config.TTL {
		return s.snapshot, nil
	}

	head, err := s.head(ctx)
	if err != nil {
		return s.snapshot, err
	}
	s.checked = time.Now()

	if head == s.snapshot.Head {
		return s.snapshot, nil
	}

	snapshot, err := s.read(ctx, head)
	if err != nil {
		return s.snapshot, err
	}
	s.snapshot = snapshot
	return s.snapshot, nil
}

func (s *Source) request(ctx context.Context, url, accept string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", accept)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "tacku")
	if s.config.Token != "" {
		request.Header.Set("Authorization", "Bearer "+s.config.Token)
	}

	response, err := s.config.Client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		// The status and nothing from the body: a refusal from the source may quote the request,
		// and the request carries a credential.
		return nil, fmt.Errorf("docsboard: %s answered %s", path.Base(url), response.Status)
	}
	return response, nil
}

func (s *Source) head(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/commits/%s", s.config.API, s.config.Repo, s.config.Ref)
	response, err := s.request(ctx, url, "application/vnd.github.sha")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	sha, err := io.ReadAll(io.LimitReader(response.Body, 64))
	if err != nil {
		return "", err
	}
	if len(strings.TrimSpace(string(sha))) == 0 {
		return "", fmt.Errorf("docsboard: the source named no commit for %q", s.config.Ref)
	}
	return strings.TrimSpace(string(sha)), nil
}

// maxArchive bounds what is read out of the archive, decompressed.
//
// A bound rather than trust: the archive is a whole repository, the size is decided elsewhere, and
// a gzip stream says nothing about what it expands to until it has expanded.
const maxArchive = 64 << 20

func (s *Source) read(ctx context.Context, head string) (Snapshot, error) {
	url := fmt.Sprintf("%s/repos/%s/tarball/%s", s.config.API, s.config.Repo, head)
	response, err := s.request(ctx, url, "application/vnd.github+json")
	if err != nil {
		return Snapshot{}, err
	}
	defer response.Body.Close()

	unzipped, err := gzip.NewReader(io.LimitReader(response.Body, maxArchive))
	if err != nil {
		return Snapshot{}, fmt.Errorf("docsboard: the archive did not open: %w", err)
	}
	defer unzipped.Close()

	snapshot := Snapshot{Head: head, TakenAt: time.Now()}
	var index string

	archive := tar.NewReader(unzipped)
	for {
		entry, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Snapshot{}, fmt.Errorf("docsboard: the archive broke off: %w", err)
		}
		if entry.Typeflag != tar.TypeReg {
			continue
		}

		// Every path in a repository archive starts with a directory named after the commit.
		name := strings.TrimPrefix(entry.Name, "./")
		_, name, ok := strings.Cut(name, "/")
		if !ok {
			continue
		}

		switch {
		case name == s.config.Index:
			body, err := io.ReadAll(io.LimitReader(archive, maxArchive))
			if err != nil {
				return Snapshot{}, err
			}
			index = string(body)
		case path.Dir(name) == s.config.Root && itemFile.MatchString(path.Base(name)):
			body, err := io.ReadAll(io.LimitReader(archive, maxArchive))
			if err != nil {
				return Snapshot{}, err
			}
			item, err := ParseItem(name, string(body))
			if err != nil {
				return Snapshot{}, err
			}
			snapshot.Items = append(snapshot.Items, item)
		}
	}

	// Nothing found is a refusal and not an empty board. Two settings decide what is read — the
	// repository and the path inside it — and a typo in either produces exactly the picture of a
	// backlog where everything is finished.
	if len(snapshot.Items) == 0 {
		return Snapshot{}, fmt.Errorf("docsboard: no items under %q at %s", s.config.Root, head[:min(7, len(head))])
	}

	snapshot.Title = title(index)
	snapshot.Stages = Stages(index, snapshot.Items)
	return snapshot, nil
}

func title(index string) string {
	for _, line := range strings.Split(index, "\n") {
		if heading, ok := strings.CutPrefix(strings.TrimSpace(line), "# "); ok {
			return strings.TrimSpace(heading)
		}
	}
	return "Backlog"
}
