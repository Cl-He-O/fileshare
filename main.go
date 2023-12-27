package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"golang.org/x/crypto/sha3"
)

//go:embed upload.html
var upload []byte

//go:embed help.txt
var help string

var b64 = base64.RawURLEncoding

type Access struct {
	Token      string `json:"token"`
	Until      int64  `json:"until"`
	MaxSize    int64  `json:"max_size"`
	Permission string `json:"permission"`
}

func sign(key []byte, access Access) string {
	access_b, err := json.Marshal(access)
	if err != nil {
		panic(err)
	}

	access_s := b64.EncodeToString(access_b)
	sig := b64.EncodeToString(sign_s(key, access_s))
	return fmt.Sprintf("?sig=%s&access=%s", sig, access_s)
}

func sign_s(key []byte, s string) []byte {
	sig := sha3.Sum256(append(key, s...))
	return sig[:]
}

func add_flag(p *string, short string, long string, value string) {
	flag.StringVar(p, short, value, "")
	flag.StringVar(p, long, value, "")
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	if len(os.Args) <= 1 || os.Args[1] == "help" {
		fmt.Println(help)
		return
	}

	var (
		config_path string
		key_s       string
		token       string
		permission  string
		duration_s  string
		max_size_s  string
	)

	add_flag(&config_path, "c", "config", "config.json")
	add_flag(&key_s, "k", "key", "")
	add_flag(&token, "t", "token", "")
	add_flag(&permission, "p", "permission", "w")
	add_flag(&duration_s, "d", "duration", "10m")
	add_flag(&max_size_s, "s", "size", "10MB")

	flag.CommandLine.Parse(os.Args[2:])

	config := parse_config(config_path)

	switch os.Args[1] {
	case "run":
		{
			serve(config)
		}
	case "new":
		{
			if key_s != "" {
				key, err := b64.DecodeString(key_s)
				if err != nil {
					panic(err)
				}

				config.key = key
			}

			if token == "" {
				t := make([]byte, 8)
				rand.Read(t)
				token = b64.EncodeToString(t)
			}

			duration, err := time.ParseDuration(duration_s)
			if err != nil {
				panic(err)
			}

			max_size, err := humanize.ParseBytes(max_size_s)
			if err != nil {
				panic(err)
			}

			access := Access{Token: token, Until: time.Now().Add(duration).Unix(), MaxSize: int64(max_size), Permission: permission}

			if permission == "w" {
				config.URL += "upload"
			} else if permission != "r" {
				fmt.Printf("unsupported permission \"%s\", should be either \"w\" or \"r\"\n", permission)
				return
			}

			fmt.Println(config.URL + sign(config.key, access))
		}
	}
}
