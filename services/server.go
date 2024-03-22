package services

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"

	"go.etcd.io/bbolt"
)

const (
	PERMISSION_WRITE Permission = "w"
	PERMISSION_READ  Permission = "r"
)

type Permission = string

//go:embed web/upload.html
var upload []byte

//go:embed web/generate.html
var generate []byte

//go:embed web/form.css
var form []byte

type Access struct {
	Token      string `json:"t"`
	Until      int64  `json:"u"`
	MaxSize    int64  `json:"s"`
	Permission string `json:"p"`

	path string
}

type FileMetadata struct {
	Filename string `json:"f"`
	Preview  bool   `json:"p"`
	Expire   int64  `json:"e"`
}

func Serve(config Config) {
	// constant
	metabucket := []byte("metadata")

	// storage
	var storage Storage

	if config.Storage.Type == STORAGE_TYPE_MINIO {
		storage = &StorageMinio{config: config.Storage.ConfigMinio}
	} else if config.Storage.Type == STORAGE_TYPE_LOCAL {
		storage = &StorageLocal{path: config.Storage.ConfigLocal.Path}
	} else {
		slog.Error("invalid storage type, should be minio/local", "storage type", config.Storage.Type)
		panic(fmt.Errorf(""))
	}

	err := storage.init()
	if err != nil {
		slog.Error("failed init storage")
		panic(err)
	}

	// database
	db, err := bbolt.Open(config.DatabasePath, 0644, bbolt.DefaultOptions)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// create bucket
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(metabucket)
		return err
	})
	if err != nil {
		panic(err)
	}

	// lock
	access_lock := NewMapRW()

	// delete expired file
	go delete_expired(db, storage, metabucket, access_lock)

	// handle delete & upload file
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		access := validate_access(config.Users, r, PERMISSION_WRITE)
		if access == nil {
			respond_text(w, http.StatusForbidden, "forbidden")
			return
		}

		if r.Method == http.MethodDelete || r.Method == http.MethodPost {
			ok := access_lock.TryLock(access.path)
			if !ok {
				msg := "there is currently a upload/download session"
				respond_message(w, msg, false)
				return
			}
			defer access_lock.Unlock(access.path)
		}

		if r.Method == http.MethodDelete {
			err := storage.delete(access.path)
			err_db := db.Update(func(tx *bbolt.Tx) error {
				return tx.Bucket(metabucket).Delete([]byte(access.path))
			})

			if err != nil {
				slog.Error("storage.delete", "err", err)
				respond_message(w, "failed to delete file", false)
			} else if err_db != nil {
				slog.Error("delete record", "err", err)
				respond_message(w, "failed to delete record", false)
			} else {
				respond_message(w, "", true)
			}

		} else if r.Method == http.MethodPost {
			preview := false

			preview_s := r.URL.Query().Get("preview")
			if preview_s == "true" {
				preview = true
			}

			r.Body = http.MaxBytesReader(w, r.Body, access.MaxSize)

			f, h, err := r.FormFile("file")
			if err != nil {
				slog.Error("FormFile", "err", err)
				respond_message(w, fmt.Sprintf("could not get form file: %s", err), false)
				return
			}

			slog.Info("storing", "filename", h.Filename)

			err = db.Update(func(tx *bbolt.Tx) error {
				v, _ := json.Marshal(FileMetadata{Filename: h.Filename, Preview: preview, Expire: access.Until})
				return tx.Bucket(metabucket).Put([]byte(access.path), v)
			})
			if err != nil {
				slog.Error("create record", "err", err)
				respond_message(w, "failed to create record", false)
				return
			}

			err = storage.put(access.path, f)
			if err != nil {
				slog.Error("storage.put failed, rolling back record", "err", err)
				db.Update(func(tx *bbolt.Tx) error {
					return tx.Bucket(metabucket).Delete([]byte(access.path))
				})

				respond_message(w, "failed to store to storage", false)
				return
			}

			respond_message(w, "", true)
		} else if r.Method == http.MethodGet {
			content_handler(".html", upload)(w, r)
		}
	})

	// get file
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		access := validate_access(config.Users, r, PERMISSION_READ)
		if access == nil {
			respond_text(w, http.StatusForbidden, "forbidden")
			return
		}

		if r.Method == http.MethodGet {
			ok := access_lock.TryRLock(access.path)
			if !ok {
				respond_text(w, http.StatusTeapot, "there is currently a upload session")
				return
			}
			defer access_lock.RUnlock(access.path)

			modtime, object, err := storage.get(access.path)

			if err != nil {
				slog.Warn("storage.get", "err", err)
				respond_text(w, http.StatusInternalServerError, "Could not get object")
				return
			}

			m := &FileMetadata{}

			err = db.View(func(tx *bbolt.Tx) error {
				v := tx.Bucket(metabucket).Get([]byte(access.path))
				if v == nil {
					return fmt.Errorf("key does not exist")
				}

				err := json.Unmarshal(v, m)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				slog.Error("get record", "err", err)
				respond_text(w, http.StatusInternalServerError, "Could not get record")
				return
			}

			if !m.Preview {
				w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": m.Filename}))
			}

			http.ServeContent(w, r, "", modtime, object)
		}
	})

	http.HandleFunc("/gen", content_handler(".html", generate))
	http.HandleFunc("/gen/form.css", content_handler(".css", form))

	err = http.ListenAndServe(config.Listen, nil)
	slog.Error("ListenAndServe", "err", err)
}
