package services

import (
	"encoding/json"
	"log/slog"
	"time"

	"go.etcd.io/bbolt"
)

func delete_expired(db *bbolt.DB, storage Storage, bucket []byte, access_lock *MapRW) {
	for {
		db.Update(func(tx *bbolt.Tx) error {
			c := tx.Bucket(bucket).Cursor()

			m := &FileMetadata{}
			k, v := c.First()
			for k != nil {
				json.Unmarshal(v, m)
				if time.Now().Unix() >= m.Expire {
					if access_lock.TryLock(string(k)) {
						path := k

						record_done := make(chan bool, 1)

						go func() {
							err := storage.delete(string(path))
							if err != nil {
								slog.Error("expire: storage.delete failed", "err", err)
							}

							<-record_done

							access_lock.Unlock(string(path))
						}()
						err := c.Delete()
						if err != nil {
							slog.Error("expire: delete record failed", "err", err)
						}

						record_done <- true
					}
				}

				k, v = c.Next()
			}

			return nil
		})

		time.Sleep(time.Second * 30)
	}
}
