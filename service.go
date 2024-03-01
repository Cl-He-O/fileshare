package main

import (
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"time"

	"github.com/minio/minio-go"
)

const (
	PERMISSION_WRITE permission = "w"
	PERMISSION_READ  permission = "r"
)

type permission = string

//go:embed web/upload.html
var upload []byte

//go:embed web/generate.html
var generate []byte

//go:embed web/form.css
var form []byte

func validate_access(users map[string][]byte, r *http.Request, perm_required permission) (*Access, string) {
	q := r.URL.Query()

	msg_forbidden := "Forbidden"

	username := q.Get("username")
	key, ok := users[username]
	if !ok {
		slog.Debug("invalid username")
		return nil, msg_forbidden
	}

	csig_s := q.Get("sig")
	access_s := q.Get("access")

	csig, err := b64.DecodeString(csig_s)
	if err != nil {
		slog.Debug("csig DecodeString", "err", err)
		return nil, msg_forbidden
	}

	sig := sign_s(key, access_s)
	if subtle.ConstantTimeCompare(sig, csig) != 1 {
		slog.Debug("invalid signature")
		return nil, msg_forbidden
	}

	access_b, err := b64.DecodeString(access_s)
	if err != nil {
		slog.Debug("access_b DecodeString", "err", err)
		return nil, msg_forbidden
	}

	var access Access
	err = json.Unmarshal(access_b, &access)
	if err != nil {
		slog.Debug("Unmarshal", "err", err)
		return nil, msg_forbidden
	}

	if time.Now().Unix() > access.Until {
		slog.Debug("token expired")
		return nil, msg_forbidden
	}

	if perm_required != access.Permission {
		slog.Debug("no permission", "perm_required", perm_required, "access.Permission", access.Permission)
		return nil, msg_forbidden
	}

	if filepath.Base(access.Token) != access.Token {
		slog.Debug("invalid token", "access.Token", access.Token)
		return nil, "Token is not a valid filename"
	}

	access.path = filepath.Join(username, access.Token)

	slog.Info("new access", "access.path", access.path, "access.Permission", access.Permission)

	return &access, ""
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
		var access *Access

		if r.Method == http.MethodDelete || r.Method == http.MethodPost || r.Method == http.MethodGet {
			var err string
			access, err = validate_access(config.users, r, PERMISSION_WRITE)
			if access == nil {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(err))
				return
			}
		}
		// access validated

		if r.Method == http.MethodDelete {
			err = mio.RemoveObject(config.Minio.Bucket, access.path)

			response := make(map[string]any)

			if err != nil {
				msg := "failed to delete file"
				slog.Error("RemoveObject", "err", err)

				response["message"] = msg
			} else {
				response["success"] = true
			}

			resp, _ := json.Marshal(response)
			w.Write(resp)

		} else if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, access.MaxSize)

			f, h, err := r.FormFile("file")
			if err != nil {
				slog.Error("FormFile", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("Could not get form file: %s", err)))
				return
			}

			slog.Info("uploading", "filename", h.Filename)

			cd := mime.FormatMediaType("attachment", map[string]string{"filename": h.Filename})
			opts := minio.PutObjectOptions{ContentDisposition: cd, ContentType: h.Header.Get("Content-Type")}

			response := make(map[string]any)

			_, err = mio.PutObject(config.Minio.Bucket, access.path, f, h.Size, opts)
			if err != nil {
				msg := "failed uploading to storage"
				slog.Error(msg, "err", err)
				response["message"] = msg
			} else {
				response["success"] = true
			}

			resp, _ := json.Marshal(response)
			w.Write(resp)
		} else if r.Method == http.MethodGet {
			content_handler(".html", upload)(w, r)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			access, err_ := validate_access(config.users, r, PERMISSION_READ)
			if access == nil {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(err_))
				return
			}

			// access validated

			object, err := mio.GetObject(config.Minio.Bucket, access.path, minio.GetObjectOptions{})
			if err != nil {
				slog.Error("GetObject", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("Could not get object: %s", err)))
				return
			}

			info, err := object.Stat()
			if err != nil {
				slog.Error("Stat", "err", err)
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(fmt.Sprintf("Could not stat object: %s", err)))
				return
			}

			w.Header().Set("Content-Type", info.ContentType)
			w.Header().Set("Content-Disposition", info.Metadata.Get("Content-Disposition"))

			http.ServeContent(w, r, "", info.LastModified, object)
		}
	})

	http.HandleFunc("/gen", content_handler(".html", generate))
	http.HandleFunc("/gen/form.css", content_handler(".css", form))

	err = http.ListenAndServe(config.Listen, nil)
	slog.Error("ListenAndServe", "err", err)
}
