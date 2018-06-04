package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
	"github.com/apoydence/triple-c/internal/handlers"
)

type TT struct {
	*testing.T
	h        *handlers.Transfer
	recorder *httptest.ResponseRecorder
	dataDir  string
}

func TestTransfer(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TT {
		dataDir, err := ioutil.TempDir("", "")
		Expect(t, err).To(BeNil())

		return TT{
			T:        t,
			dataDir:  dataDir,
			h:        handlers.NewTransfer("http://some.url", dataDir, log.New(ioutil.Discard, "", 0)),
			recorder: httptest.NewRecorder(),
		}
	})

	o.Spec("writes data from the POST to the dataDir", func(t TT) {
		name := t.h.InitInterconnect(context.Background())
		expectedData := make([]byte, 10*1024)
		rand.Read(expectedData)

		postReq, err := http.NewRequest("POST", name, bytes.NewReader(expectedData))
		Expect(t, err).To(BeNil())
		recorder := httptest.NewRecorder()
		t.h.ServeHTTP(recorder, postReq)
		Expect(t, recorder.Code).To(Equal(http.StatusOK))

		name = path.Base(name)

		f, err := os.Open(path.Join(t.dataDir, name))
		Expect(t, err).To(BeNil())
		defer f.Close()

		data, err := ioutil.ReadAll(f)
		Expect(t, err).To(BeNil())
		Expect(t, data).To(Equal(expectedData))
	})

	o.Spec("GET reads data from the dataDir", func(t TT) {
		name := t.h.InitInterconnect(context.Background())
		expectedData := make([]byte, 10*1024)
		rand.Read(expectedData)

		f, err := os.Create(path.Join(t.dataDir, path.Base(name)))
		Expect(t, err).To(BeNil())
		io.Copy(f, bytes.NewReader(expectedData))
		f.Close()

		getReq, err := http.NewRequest("GET", name, bytes.NewReader(nil))
		Expect(t, err).To(BeNil())
		recorder := httptest.NewRecorder()
		t.h.ServeHTTP(recorder, getReq)
		Expect(t, recorder.Code).To(Equal(http.StatusOK))
		Expect(t, recorder.Body.Bytes()).To(Equal(expectedData))
	})

	o.Spec("it returns a 405 for non GET or POST", func(t TT) {
		name := t.h.InitInterconnect(context.Background())
		req, err := http.NewRequest("PUT", name, bytes.NewReader(nil))
		Expect(t, err).To(BeNil())

		t.h.ServeHTTP(t.recorder, req)
		Expect(t, t.recorder.Code).To(Equal(http.StatusMethodNotAllowed))
	})

	o.Spec("InitInterconnect returns unique names", func(t TT) {
		a := t.h.InitInterconnect(context.Background())
		b := t.h.InitInterconnect(context.Background())
		Expect(t, a).To(Not(Equal(b)))
	})

	o.Spec("it returns a 404 for an expired name", func(t TT) {
		ctx, cancel := context.WithCancel(context.Background())
		name := t.h.InitInterconnect(ctx)
		req, err := http.NewRequest("GET", name, bytes.NewReader(nil))
		Expect(t, err).To(BeNil())

		cancel()
		time.Sleep(100 * time.Millisecond)

		t.h.ServeHTTP(t.recorder, req)
		Expect(t, t.recorder.Code).To(Equal(http.StatusNotFound))
	})

	o.Spec("it returns a 404 for an unknown name", func(t TT) {
		req, err := http.NewRequest("GET", "http://some.url/v1/transfer/unknown", bytes.NewReader(nil))
		Expect(t, err).To(BeNil())

		t.h.ServeHTTP(t.recorder, req)
		Expect(t, t.recorder.Code).To(Equal(http.StatusNotFound))
	})

	o.Spec("it survives the race detector", func(t TT) {
		go func() {
			for i := 0; i < 100; i++ {
				ctx, _ := context.WithTimeout(context.Background(), time.Microsecond)
				req, err := http.NewRequest("GET", t.h.InitInterconnect(ctx), bytes.NewReader(nil))
				Expect(t, err).To(BeNil())
				t.h.ServeHTTP(httptest.NewRecorder(), req)
			}
		}()

		for i := 0; i < 100; i++ {
			ctx, _ := context.WithTimeout(context.Background(), time.Microsecond)
			req, err := http.NewRequest("GET", t.h.InitInterconnect(ctx), bytes.NewReader(nil))
			Expect(t, err).To(BeNil())
			t.h.ServeHTTP(httptest.NewRecorder(), req)
		}
	})
}
