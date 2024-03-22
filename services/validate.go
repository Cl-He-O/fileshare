package services

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

func validate_access(users map[string][]byte, r *http.Request, perm_required Permission) *Access {
	q := r.URL.Query()

	username := q.Get("username")
	key, ok := users[username]
	if !ok {
		slog.Debug("invalid username")
		return nil
	}

	csig_s := q.Get("sig")
	access_s := q.Get("access")

	csig, err := B64.DecodeString(csig_s)
	if err != nil {
		slog.Debug("csig DecodeString", "err", err)
		return nil
	}

	sig := sign_s(key, access_s)
	if subtle.ConstantTimeCompare(sig, csig) != 1 {
		slog.Debug("invalid signature")
		return nil
	}

	access_b, err := B64.DecodeString(access_s)
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

	if time.Now().Unix() >= access.Until {
		slog.Debug("token expired")
		return nil
	}

	if perm_required != access.Permission {
		slog.Debug("no permission", "perm_required", perm_required, "access.Permission", access.Permission)
		return nil
	}

	path, _ := json.Marshal(map[string]string{"u": username, "a": access.Token})
	access.path = B64.EncodeToString(path)

	slog.Info("new access", "access.path", access.path, "access.Permission", access.Permission)

	return &access
}
