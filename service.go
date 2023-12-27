package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/minio/minio-go"
)

func decode_access(key []byte, r *http.Request) *Access {
	csig_s := r.FormValue("sig")
	access_s := r.FormValue("access")

	csig, err := b64.DecodeString(csig_s)
	if err != nil {
		slog.Debug("DecodeString", "err", err)
		return nil
	}

	sig := sign_s(key, access_s)
	if subtle.ConstantTimeCompare(sig, csig) != 1 {
		slog.Debug("invalid signature")
		return nil
	}

	access_b, err := b64.DecodeString(r.FormValue("access"))
	if err != nil {
		slog.Debug("DecodeString", "err", err)
		return nil
	}

	var access Access
	err = json.Unmarshal(access_b, &access)
	if err != nil {
		slog.Debug("Unmarshal", "err", err)
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

			if time.Now().Unix() > access.Until {
				slog.Debug("token expired")
				return
			}

			err = r.ParseMultipartForm(access.MaxSize)
			if err != nil {
				slog.Debug("ParseMultipartForm", "err", err)
				return
			}

			f, h, err := r.FormFile("file")
			if err != nil {
				slog.Debug("FormFile", "err", err)
				return
			}

			slog.Debug("uploading", "filename", h.Filename)

			cd := mime.FormatMediaType("attachment", map[string]string{"filename": h.Filename})
			opts := minio.PutObjectOptions{ContentDisposition: cd, ContentType: h.Header.Get("Content-Type")}

			mio.PutObject(config.Minio.Bucket, access.Token, f, h.Size, opts)

			resp, _ := json.Marshal(map[string]bool{"success": true})
			w.Write(resp)
		} else if r.Method == http.MethodGet {
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
				slog.Debug("GetObject", "err", err)
				return
			}

			info, err := object.Stat()
			if err != nil {
				slog.Debug("Stat", "err", err)
				return
			}

			w.Header().Set("Content-Type", info.ContentType)
			w.Header().Set("Content-Disposition", info.Metadata.Get("Content-Disposition"))

			http.ServeContent(w, r, "", time.Time{}, object)
		}
	})

	err = http.ListenAndServe(config.Listen, nil)
	slog.Error("ListenAndServe", "err", err)
}
