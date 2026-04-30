package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/KuraDB/internal/database"
	goUtils_filesystem "github.com/pardnchiu/go-utils/filesystem"
)

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `usage:
  kura                            start server (DB_NAME from env / .env)
  kura add <name>                 register a new db
  kura list                       list registered dbs
  kura remove <name>              unregister and delete a db (requires confirmation)
  kura edit <old> <new>           rename a db
  kura help                       show this message`)
}

func cmdAdd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: kura add <db name>")
		os.Exit(2)
	}
	name := sanitizeDBName(args[0])
	if name == "" {
		fmt.Fprintln(os.Stderr, "add: invalid name")
		os.Exit(2)
	}

	homeDir, configDir := mustConfigDir()
	reg := database.New(filepath.Join(configDir, "db.json"))

	has, err := reg.Has(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "add: registry.Has: %v\n", err)
		os.Exit(1)
	}
	if has {
		fmt.Fprintf(os.Stderr, "add: db already registered: %s\n", name)
		os.Exit(1)
	}

	dbDir := filepath.Join(configDir, name)
	if _, err := os.Stat(dbDir); err == nil {
		fmt.Fprintf(os.Stderr, "add: directory already exists: %s\n", dbDir)
		os.Exit(1)
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "add: stat: %v\n", err)
		os.Exit(1)
	}

	folderDir := filepath.Join(dbDir, "inbox")
	if err := goUtils_filesystem.CheckDir(folderDir, true); err != nil {
		fmt.Fprintf(os.Stderr, "add: CheckDir: %v\n", err)
		os.Exit(1)
	}

	linkPath := filepath.Join(homeDir, "Kura_"+name)
	if err := ensureSymlink(folderDir, linkPath); err != nil {
		fmt.Fprintf(os.Stderr, "add: ensureSymlink: %v\n", err)
		os.Exit(1)
	}

	if err := reg.Add(name); err != nil {
		fmt.Fprintf(os.Stderr, "add: registry.Add: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("db added: %s\n  dir:  %s\n  link: %s\n", name, dbDir, linkPath)
}

func cmdList(_ []string) {
	_, configDir := mustConfigDir()
	reg := database.New(filepath.Join(configDir, "db.json"))

	entries, err := reg.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "list: registry.Load: %v\n", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println("(no registered db)")
		return
	}
	for _, e := range entries {
		fmt.Printf("%s\t%s\n", e.DB, e.CreateAt)
	}
}

func cmdRemove(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: kura remove <db name>")
		os.Exit(2)
	}
	name := sanitizeDBName(args[0])
	if name == "" {
		fmt.Fprintln(os.Stderr, "remove: invalid name")
		os.Exit(2)
	}

	homeDir, configDir := mustConfigDir()
	reg := database.New(filepath.Join(configDir, "db.json"))

	has, err := reg.Has(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "remove: registry.Has: %v\n", err)
		os.Exit(1)
	}
	if !has {
		fmt.Fprintf(os.Stderr, "remove: db not registered: %s\n", name)
		os.Exit(1)
	}

	dbDir := filepath.Join(configDir, name)
	linkPath := filepath.Join(homeDir, "Kura_"+name)

	fmt.Printf("Permanently delete db %q?\n  %s\n  %s\nType 'yes' to confirm: ", name, dbDir, linkPath)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "remove: read confirm: %v\n", err)
		os.Exit(1)
	}
	if strings.TrimSpace(line) != "yes" {
		fmt.Println("aborted")
		return
	}

	if err := os.RemoveAll(dbDir); err != nil {
		fmt.Fprintf(os.Stderr, "remove: RemoveAll: %v\n", err)
		os.Exit(1)
	}

	if info, err := os.Lstat(linkPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(linkPath); err != nil {
				fmt.Fprintf(os.Stderr, "remove: os.Remove symlink: %v\n", err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "remove: skip non-symlink at link path: %s\n", linkPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "remove: lstat symlink: %v\n", err)
	}

	if err := reg.Remove(name); err != nil {
		fmt.Fprintf(os.Stderr, "remove: registry.Remove: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("db removed: %s\n", name)
}

func cmdEdit(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: kura edit <old name> <new name>")
		os.Exit(2)
	}
	oldName := sanitizeDBName(args[0])
	newName := sanitizeDBName(args[1])
	if oldName == "" || newName == "" {
		fmt.Fprintln(os.Stderr, "edit: invalid name")
		os.Exit(2)
	}
	if oldName == newName {
		fmt.Println("noop")
		return
	}

	homeDir, configDir := mustConfigDir()
	reg := database.New(filepath.Join(configDir, "db.json"))

	has, err := reg.Has(oldName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "edit: registry.Has: %v\n", err)
		os.Exit(1)
	}
	if !has {
		fmt.Fprintf(os.Stderr, "edit: db not registered: %s\n", oldName)
		os.Exit(1)
	}
	hasNew, err := reg.Has(newName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "edit: registry.Has: %v\n", err)
		os.Exit(1)
	}
	if hasNew {
		fmt.Fprintf(os.Stderr, "edit: target name already exists: %s\n", newName)
		os.Exit(1)
	}

	oldDir := filepath.Join(configDir, oldName)
	newDir := filepath.Join(configDir, newName)
	if _, err := os.Stat(newDir); err == nil {
		fmt.Fprintf(os.Stderr, "edit: target directory already exists: %s\n", newDir)
		os.Exit(1)
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "edit: stat target: %v\n", err)
		os.Exit(1)
	}

	if err := os.Rename(oldDir, newDir); err != nil {
		fmt.Fprintf(os.Stderr, "edit: rename folder %s -> %s: %v\n", oldDir, newDir, err)
		os.Exit(1)
	}

	oldLink := filepath.Join(homeDir, "Kura_"+oldName)
	if info, err := os.Lstat(oldLink); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(oldLink); err != nil {
				fmt.Fprintf(os.Stderr, "edit: remove old symlink: %v\n", err)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "edit: lstat old symlink: %v\n", err)
	}

	newLink := filepath.Join(homeDir, "Kura_"+newName)
	if err := ensureSymlink(filepath.Join(newDir, "inbox"), newLink); err != nil {
		fmt.Fprintf(os.Stderr, "edit: ensureSymlink: %v\n", err)
	}

	if err := reg.Rename(oldName, newName); err != nil {
		fmt.Fprintf(os.Stderr, "edit: registry.Rename: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("db renamed: %s -> %s\n", oldName, newName)
}
