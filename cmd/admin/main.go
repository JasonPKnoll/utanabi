// Command admin manages who's an admin on a server, and the server's YouTube API key. Admins make invites in the app;
// being one is set only here, on the server's own machine, so nobody can make themselves one through the app.
//
//	go run ./cmd/admin -list
//	go run ./cmd/admin -grant <username>
//	go run ./cmd/admin -revoke <username>
//	go run ./cmd/admin -youtube-key < key.txt
//	go run ./cmd/admin -youtube-key-clear
//
// -youtube-key reads the key from standard input, so it isn't kept in the shell's history, checks it with Google,
// and saves it sealed with the server's secret.key, as entering it in Settings does. make youtube-key makes one and
// saves it this way.
//
// It works on utanabi.db in -data, which defaults to UTANABI_DATA_DIR, then ./data. A running server sees the change
// on the user's next request.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/secretbox"
	"github.com/JasonPKnoll/utanabi/internal/features/accounts"
	"github.com/JasonPKnoll/utanabi/internal/features/youtube"
	"github.com/JasonPKnoll/utanabi/internal/identity"
	"github.com/JasonPKnoll/utanabi/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "admin:", err)
		os.Exit(1)
	}
}

func run() (err error) {
	dataDir := os.Getenv("UTANABI_DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	data := flag.String("data", dataDir, "the server's data folder, which holds utanabi.db")
	list := flag.Bool("list", false, "list the accounts and who's an admin")
	grant := flag.String("grant", "", "make this user an admin")
	revoke := flag.String("revoke", "", "make this user no longer an admin")
	setKey := flag.Bool("youtube-key", false, "check the YouTube API key on standard input, and save it")
	clearKey := flag.Bool("youtube-key-clear", false, "forget the YouTube API key saved before")
	flag.Parse()
	if !*list && *grant == "" && *revoke == "" && !*setKey && !*clearKey {
		flag.Usage()
		return errors.New("say -list, -grant <username>, -revoke <username>, -youtube-key, or -youtube-key-clear")
	}

	ctx := context.Background()
	path := filepath.Join(*data, "utanabi.db")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("no database at %s (%v)", path, err)
	}
	db, err := store.Open(ctx, path, 5*time.Second)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	if err := db.Migrate(ctx, store.Migrations()); err != nil {
		return err
	}
	repo := accounts.NewSQLite(db)

	if *setKey || *clearKey {
		box, err := secretbox.LoadOrCreate(filepath.Join(*data, "secret.key"))
		if err != nil {
			return err
		}
		keys := youtube.NewSQLite(db, identity.Accounts(), box)
		if *clearKey {
			if err := keys.DeleteAPIKey(ctx); err != nil {
				return err
			}
			fmt.Println("The YouTube API key is gone; songs no longer search YouTube unless the server sets one.")
		}
		if *setKey {
			line, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil && line == "" {
				return fmt.Errorf("no key on standard input (%v)", err)
			}
			if err := youtube.SaveKey(ctx, keys, youtube.NewDataAPI(youtube.APIURL), line, time.Now()); err != nil {
				return err
			}
			fmt.Println("The YouTube API key is saved, sealed; songs now find their videos on YouTube.")
		}
	}

	for name, admin := range map[string]bool{*grant: true, *revoke: false} {
		if name == "" {
			continue
		}
		canonical, err := accounts.Name(name)
		if err != nil {
			return err
		}
		if err := repo.SetAdmin(ctx, canonical, admin); errors.Is(err, fault.ErrNotFound) {
			return fmt.Errorf("there's no account called %q", canonical)
		} else if err != nil {
			return err
		}
		fmt.Printf("%s is %s\n", canonical, map[bool]string{true: "an admin", false: "no longer an admin"}[admin])
	}

	if *list {
		users, err := repo.Users(ctx)
		if err != nil {
			return err
		}
		for _, u := range users {
			role := ""
			if u.Admin {
				role = "  admin"
			}
			fmt.Printf("%-32s joined %s%s\n", u.Name, u.CreatedAt.Format("2006-01-02 15:04"), role)
		}
	}
	return nil
}
