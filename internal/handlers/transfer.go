package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

type Transfer struct {
	mu      sync.RWMutex
	m       map[string]transferInfo
	dataDir string
	host    string
	log     *log.Logger
}

type transferInfo struct {
	c   chan []byte
	ctx context.Context
}

func NewTransfer(host, dataDir string, log *log.Logger) *Transfer {
	return &Transfer{
		m:       make(map[string]transferInfo),
		host:    host,
		dataDir: dataDir,
		log:     log,
	}
}

func (t *Transfer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	t.mu.RLock()
	defer t.mu.RUnlock()

	if !strings.HasPrefix(r.URL.Path, "/v1/transfer/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	name := r.URL.Path[len("/v1/transfer/"):]
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		c, ok := t.m[name]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		f, err := os.Open(path.Join(t.dataDir, name))
		if err != nil {
			t.log.Printf("failed to create file to transfer data: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer f.Close()

		select {
		case <-ctx.Done():
			return
		case <-c.ctx.Done():
			return
		default:
			io.Copy(w, f)
		}

	case http.MethodPost:
		c, ok := t.m[name]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		defer close(c.c)

		f, err := os.Create(path.Join(t.dataDir, name))
		if err != nil {
			t.log.Printf("failed to create file to transfer data: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer f.Close()

		_, err = io.Copy(f, r.Body)
		if err != nil {
			log.Printf("failed to save data from transfer: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (t *Transfer) InitInterconnect(ctx context.Context) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	name := fmt.Sprint(time.Now().UnixNano())
	t.m[name] = transferInfo{
		c:   make(chan []byte, 100),
		ctx: ctx,
	}

	go func() {
		<-ctx.Done()
		t.mu.Lock()
		log.Printf("done with transfer handler at %s/v1/transfer/%s", t.host, name)
		os.Remove(path.Join(t.dataDir, name))
		defer t.mu.Unlock()
		delete(t.m, name)
	}()

	log.Printf("starting transfer handler at %s/v1/transfer/%s", t.host, name)
	return fmt.Sprintf("%s/v1/transfer/%s", t.host, name)
}
