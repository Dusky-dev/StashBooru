package manager

import (
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/hash"
	"github.com/stashapp/stash/pkg/logger"
)

// DownloadStore manages single-use generated files for the UI to download.
type DownloadStore struct {
	m     map[string]*storeFile
	mutex sync.Mutex
}

type storeFile struct {
	path        string
	contentType string
	keep        bool
	wg          sync.WaitGroup
	once        sync.Once
}

func NewDownloadStore() *DownloadStore {
	return &DownloadStore{
		m: make(map[string]*storeFile),
	}
}

func (s *DownloadStore) RegisterFile(fp string, contentType string, keep bool) (string, error) {
	const keyLength = 4
	const attempts = 100

	// keep generating random keys until we get a free one
	// prevent infinite loop by only attempting a finite amount of times
	var h string
	generate := true
	a := 0

	s.mutex.Lock()
	for generate && a < attempts {
		var err error
		h, err = hash.GenerateRandomKey(keyLength)
		if err != nil {
			return "", err
		}
		_, generate = s.m[h]
		a++
	}

	s.m[h] = &storeFile{
		path:        fp,
		contentType: contentType,
		keep:        keep,
	}
	s.mutex.Unlock()

	return h, nil
}

func (s *DownloadStore) Serve(hash string, w http.ResponseWriter, r *http.Request) {
	s.mutex.Lock()
	f, ok := s.m[hash]

	if !ok {
		s.mutex.Unlock()
		http.NotFound(w, r)
		return
	}

	if !f.keep {
		s.waitAndRemoveFile(hash, &w, r)
	}

	s.mutex.Unlock()

	if f.contentType != "" {
		w.Header().Add("Content-Type", f.contentType)
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, f.path)
}

func (s *DownloadStore) waitAndRemoveFile(hash string, w *http.ResponseWriter, r *http.Request) {
	f := s.m[hash]
	notify := r.Context().Done()
	f.wg.Add(1)

	go func() {
		<-notify
		s.mutex.Lock()
		defer s.mutex.Unlock()

		f.wg.Done()
	}()

	f.once.Do(func() {
		time.AfterFunc(30*time.Second, func() { s.expireFile(hash, f) })
	})
}

// ExpireFile also releases generated files that are never fetched by a browser.
func (s *DownloadStore) ExpireFile(key string, after time.Duration) {
	s.mutex.Lock()
	f := s.m[key]
	s.mutex.Unlock()
	if f != nil {
		time.AfterFunc(after, func() { s.expireFile(key, f) })
	}
}

func (s *DownloadStore) expireFile(key string, f *storeFile) {
	// Remove registration before waiting: no new readers can join the group.
	// Compare the entry too, so an old timer cannot evict a reused random key.
	s.mutex.Lock()
	if s.m[key] != f {
		s.mutex.Unlock()
		return
	}
	delete(s.m, key)
	s.mutex.Unlock()
	f.wg.Wait()
	if err := os.Remove(f.path); err != nil && !os.IsNotExist(err) {
		logger.Errorf("error removing generated download: %s", err)
	}
}
