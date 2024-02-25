package main

import (
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/minio/minio-go"
)

//go:embed web/upload.html
var upload []byte

//go:embed web/generate.html
var generate []byte

func decode_access(key []byte, r *http.Request) *Access {
	q := r.URL.Query()

	csig_s := q.Get("sig")
	access_s := q.Get("access")

	csig, err := b64.DecodeString(csig_s)
	if err != nil {
		slog.Debug("csig DecodeString", "err", err)
		return nil
	}

	sig := sign_s(key, access_s)
	if subtle.ConstantTimeCompare(sig, csig) != 1 {
		slog.Debug("invalid signature")
		return nil
	}

	access_b, err := b64.DecodeString(access_s)
	if err != nil {
		slog.Debug("access_b DecodeString", "err", err)
		return nil
	}

	var access Access
	err = json.Unmarshal(access_b, &access)
	if err != nil {
		slog.Debug("Unmarshal", "err", err)
		return nil
	}

	if time.Now().Unix() > access.Until {
		slog.Debug("token expired")
		return nil
	}

	return &access
}

func serve(config Config) {
	mio, err := minio.New(config.Minio.Endpoint, config.Minio.ID, config.Minio.Secret, config.Minio.UseSSL)
	if err != nil {
		panic(err)
	}

	exist, err := mio.BucketExists(config.Minio.Bucket)
	if err != nil {
		panic(err)
	}
	if !exist {
		panic(fmt.Sprintf("bucket \"%s\" doesn't exist", config.Minio.Bucket))
	}

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			access := decode_access(config.key, r)
			if access == nil {
				return
			}

			if access.Permission != "w" {
				slog.Debug("no write permission")
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, access.MaxSize)

			f, h, err := r.FormFile("file")
			if err != nil {
				slog.Debug("FormFile", "err", err)
				return
			}

			slog.Debug("uploading", "filename", h.Filename)

			cd := mime.FormatMediaType("attachment", map[string]string{"filename": h.Filename})
			opts := minio.PutObjectOptions{ContentDisposition: cd, ContentType: h.Header.Get("Content-Type")}

			response := make(map[string]any)

			_, err = mio.PutObject(config.Minio.Bucket, access.Token, f, h.Size, opts)
			if err != nil {
				msg := "failed uploading to storage"
				slog.Warn(msg, "err", err)
				response["message"] = msg
			} else {
				response["success"] = true
			}

			resp, _ := json.Marshal(response)
			w.Write(resp)
		} else if r.Method == http.MethodGet {
			access := decode_access(config.key, r)
			if access == nil {
				return
			}

			w.Write(upload)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			access := decode_access(config.key, r)
			if access == nil {
				return
			}

			if access.Permission != "r" {
				slog.Debug("no read permission")
				return
			}

			object, err := mio.GetObject(config.Minio.Bucket, access.Token, minio.GetObjectOptions{})
			if err != nil {
				slog.Warn("GetObject", "err", err)
				return
			}

			info, err := object.Stat()
			if err != nil {
				slog.Warn("Stat", "err", err)
				return
			}

			w.Header().Set("Content-Type", info.ContentType)
			w.Header().Set("Content-Disposition", info.Metadata.Get("Content-Disposition"))

			http.ServeContent(w, r, "", time.Time{}, object)
		}
	})

	http.HandleFunc("/gen", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write(generate)
		}
	})

	err = http.ListenAndServe(config.Listen, nil)
	slog.Error("ListenAndServe", "err", err)
}
