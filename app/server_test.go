package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/google/go-cmp/cmp"

	"github.com/gilcrest/go-api-basic/domain/logger"
	"github.com/rs/zerolog"

	"github.com/gorilla/mux"

	qt "github.com/frankban/quicktest"
	"github.com/gilcrest/go-api-basic/domain/errs"
)

func TestDecoderErr(t *testing.T) {
	t.Run("typical", func(t *testing.T) {
		c := qt.New(t)

		type testBody struct {
			Director string `json:"director"`
			Writer   string `json:"writer"`
		}

		requestBody := []byte(`{
				"director": "Alex Cox",
				"writer": "Alex Cox"
			}`)

		r, err := http.NewRequest(http.MethodPost, "/fake", bytes.NewBuffer(requestBody))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}

		// Decode JSON HTTP request body into a Decoder type
		// and unmarshal that into the testBody struct. DecoderErr
		// wraps errors from Decode when body is nil, json is malformed
		// or any other error
		wantBody := new(testBody)
		err = decoderErr(json.NewDecoder(r.Body).Decode(&wantBody))
		defer r.Body.Close()
		c.Assert(err, qt.IsNil)
	})

	t.Run("malformed JSON", func(t *testing.T) {
		c := qt.New(t)

		type testBody struct {
			Director string `json:"director"`
			Writer   string `json:"writer"`
		}

		// removed trailing curly bracket
		requestBody := []byte(`{
				"director": "Alex Cox",
				"writer": "Alex Cox"`)

		r, err := http.NewRequest(http.MethodPost, "/fake", bytes.NewBuffer(requestBody))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}

		// Decode JSON HTTP request body into a Decoder type
		// and unmarshal that into the testBody struct. DecoderErr
		// wraps errors from Decode when body is nil, JSON is malformed
		// or any other error
		wantBody := new(testBody)
		err = decoderErr(json.NewDecoder(r.Body).Decode(&wantBody))
		defer r.Body.Close()

		wantErr := errs.E(errs.InvalidRequest, errors.New("Malformed JSON"))
		c.Assert(errs.Match(err, wantErr), qt.IsTrue)
	})

	t.Run("empty request body", func(t *testing.T) {
		c := qt.New(t)

		type testBody struct {
			Director string `json:"director"`
			Writer   string `json:"writer"`
		}

		// empty body
		requestBody := []byte("")

		r, err := http.NewRequest(http.MethodPost, "/fake", bytes.NewBuffer(requestBody))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}

		// Decode JSON HTTP request body into a Decoder type
		// and unmarshal that into the testBody struct. DecoderErr
		// wraps errors from Decode when body is nil, JSON is malformed
		// or any other error
		wantBody := new(testBody)
		err = decoderErr(json.NewDecoder(r.Body).Decode(&wantBody))
		defer r.Body.Close()

		wantErr := errs.E(errs.InvalidRequest, errors.New("Request Body cannot be empty"))
		c.Assert(errs.Match(err, wantErr), qt.IsTrue)
	})

	t.Run("invalid request body", func(t *testing.T) {
		c := qt.New(t)

		type testBody struct {
			Director string `json:"director"`
			Writer   string `json:"writer"`
		}

		// has unknown field
		requestBody := []byte(`{
				"director": "Alex Cox",
				"writer": "Alex Cox",
                "unknown_field": "I should fail"
			}`)

		r, err := http.NewRequest(http.MethodPost, "/fake", bytes.NewBuffer(requestBody))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}

		// force an error with DisallowUnknownFields
		wantBody := new(testBody)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		err = decoderErr(decoder.Decode(&wantBody))
		defer r.Body.Close()

		// check to make sure I have an error
		c.Assert(err != nil, qt.Equals, true)
	})
}

func TestNewServer(t *testing.T) {
	c := qt.New(t)

	type args struct {
		r      *mux.Router
		params *ServerParams
	}

	lgr := logger.NewLogger(os.Stdout, zerolog.DebugLevel, true)

	badlgr := logger.NewLogger(os.Stderr, zerolog.DebugLevel, true)

	driver := NewDriver()

	r := NewMuxRouter()
	p := NewServerParams(lgr, driver)
	p2 := NewServerParams(lgr, nil)

	typServer := &Server{
		router: r,
		driver: driver,
		logger: badlgr,
	}

	tests := []struct {
		name    string
		args    args
		want    *Server
		wantErr error
	}{
		{"typical", args{r: r, params: p}, typServer, nil},
		{"nil params", args{r: r, params: nil}, nil, errs.E("params must not be nil")},
		{"nil params.Driver", args{r: r, params: p2}, nil, errs.E("params.Driver must not be nil")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewServer(tt.args.r, tt.args.params)
			if (err != nil) && (tt.wantErr == nil) {
				t.Errorf("NewServer() error = %v, nil expected", err)
				return
			}
			c.Assert(err, qt.CmpEquals(cmp.Comparer(errs.Match)), tt.wantErr)
			c.Assert(got, qt.CmpEquals(cmpopts.IgnoreUnexported(Server{})), tt.want)
		})
	}
}
