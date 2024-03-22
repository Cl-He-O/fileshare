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

	"github.com/Cl-He-O/fileshare/services"
	"github.com/dustin/go-humanize"
)

//go:embed help.txt
var help string

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
		username    string
		token       string
		permission  string
		duration_s  string
		max_size_s  string
	)

	add_flag(&config_path, "c", "config", "config.json")
	add_flag(&username, "u", "username", "")
	add_flag(&token, "t", "token", "")
	add_flag(&permission, "p", "permission", "w")
	add_flag(&duration_s, "d", "duration", "10m")
	add_flag(&max_size_s, "s", "size", "10MB")

	flag.CommandLine.Parse(os.Args[2:])

	config := services.ParseConfig(config_path)

	switch os.Args[1] {
	case "run":
		{
			services.Serve(config)
		}
	case "new":
		{
			if token == "" {
				t := make([]byte, 8)
				rand.Read(t)
				token = services.B64.EncodeToString(t)

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

			if permission == services.PERMISSION_READ {
				max_size = 0
			}

			access := services.Access{Token: token, Until: time.Now().Add(duration).Unix(), MaxSize: int64(max_size), Permission: permission}

			u, err := url.Parse(config.URL)
			if err != nil {
				panic(err)
			}

			if permission == services.PERMISSION_WRITE {
				u = u.JoinPath("upload")
			} else if permission != services.PERMISSION_READ {
				panic(fmt.Errorf("unsupported permission \"%s\", should be either \"w\" or \"r\"", permission))
			}

			key, ok := config.Users[username]
			if !ok {
				panic(fmt.Errorf("invalid username \"%s\"", username))
			}

			u.RawQuery = services.Sign(username, key, access).Encode()
			fmt.Println(u)
		}
	}
}
