package main

import (
	"crypto/rand"
	_ "embed"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/dustin/go-humanize"
)

//go:embed help.txt
var help string

type Access struct {
	Token      string `json:"t"`
	Until      int64  `json:"u"`
	MaxSize    int64  `json:"s"`
	Permission string `json:"p"`
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	if len(os.Args) <= 1 || os.Args[1] == "help" {
		fmt.Println(help)
		return
	}

	var (
		config_path string
		token       string
		permission  string
		duration_s  string
		max_size_s  string
	)

	add_flag(&config_path, "c", "config", "config.json")
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
			if token == "" {
				t := make([]byte, 8)
				rand.Read(t)
				token = b64.EncodeToString(t)

				fmt.Fprintln(os.Stderr, token)
			}

			duration, err := time.ParseDuration(duration_s)
			if err != nil {
				panic(err)
			}

			max_size, err := humanize.ParseBytes(max_size_s)
			if err != nil {
				panic(err)
			}

			if permission == "r" {
				max_size = 0
			}

			access := Access{Token: token, Until: time.Now().Add(duration).Unix(), MaxSize: int64(max_size), Permission: permission}

			u, err := url.Parse(config.URL)
			if err != nil {
				panic(err)
			}

			if permission == "w" {
				u = u.JoinPath("upload")
			} else if permission != "r" {
				panic(fmt.Sprintf("unsupported permission \"%s\", should be either \"w\" or \"r\"\n", permission))
			}

			u.RawQuery = sign(config.key, access).Encode()
			fmt.Println(u)
		}
	}
}
