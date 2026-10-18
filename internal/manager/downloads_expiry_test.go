package manager

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDownloadExpiryCleansUnusedAndWaitsForReaders(t *testing.T) {
	s := NewDownloadStore()
	path := filepath.Join(t.TempDir(), "original.jpg")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	key, err := s.RegisterFile(path, "image/jpeg", false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := httptest.NewRecorder()
	s.Serve(key, w, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	s.ExpireFile(key, time.Millisecond)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s.mutex.Lock()
		gone := s.m[key] == nil
		s.mutex.Unlock()
		if gone {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("active download removed", err)
	}
	w = httptest.NewRecorder()
	s.Serve(key, w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 404 {
		t.Fatal("expired registration still available")
	}
	cancel()
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("expired file not removed after reader finished")
}
