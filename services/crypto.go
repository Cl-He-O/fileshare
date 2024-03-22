package services

import (
	"encoding/json"
	"net/url"

	"golang.org/x/crypto/sha3"
)

func Sign(username string, key []byte, access Access) url.Values {
	access_b, err := json.Marshal(access)
	if err != nil {
		panic(err)
	}

	access_s := B64.EncodeToString(access_b)
	sig := B64.EncodeToString(sign_s(key, access_s))

	query := make(url.Values)
	query.Set("username", username)
	query.Set("sig", sig)
	query.Set("access", access_s)

	return query
}

func sign_s(key []byte, s string) []byte {
	sig := sha3.Sum256(append(key, s...))
	return sig[:]
}
