package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/crypto/sha3"
)

var b64 = base64.RawURLEncoding

func sign(key []byte, access Access) url.Values {
	access_b, err := json.Marshal(access)
	if err != nil {
		panic(err)
	}

	access_s := b64.EncodeToString(access_b)
	sig := b64.EncodeToString(sign_s(key, access_s))

	query := make(url.Values)
	query.Set("sig", sig)
	query.Set("access", access_s)

	return query
}

func sign_s(key []byte, s string) []byte {
	sig := sha3.Sum256(append(key, s...))
	return sig[:]
}

func add_flag(p *string, short string, long string, value string) {
	flag.StringVar(p, short, value, "")
	flag.StringVar(p, long, value, "")
}

func content_handler(name string, content []byte) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(content))
	}
}
