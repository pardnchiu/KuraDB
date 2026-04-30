package database

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	goUtils_filesystem "github.com/pardnchiu/go-utils/filesystem"
)

type Entry struct {
	DB       string `json:"db"`
	CreateAt string `json:"createAt"`
}

type Registry struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Registry {
	return &Registry{path: path}
}

func (r *Registry) Load() ([]Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.load()
}

func (r *Registry) load() ([]Entry, error) {
	list, err := goUtils_filesystem.ReadJSON[[]Entry](r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Entry{}, nil
		}
		return nil, err
	}
	if list == nil {
		return []Entry{}, nil
	}
	return list, nil
}

func (r *Registry) Has(name string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list, err := r.load()
	if err != nil {
		return false, err
	}
	for _, e := range list {
		if e.DB == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *Registry) Add(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	list, err := r.load()
	if err != nil {
		return err
	}
	for _, e := range list {
		if e.DB == name {
			return fmt.Errorf("db %q already registered", name)
		}
	}
	list = append(list, Entry{
		DB:       name,
		CreateAt: time.Now().UTC().Format(time.RFC3339),
	})
	return r.save(list)
}

func (r *Registry) AddIfMissing(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	list, err := r.load()
	if err != nil {
		return err
	}
	for _, e := range list {
		if e.DB == name {
			return nil
		}
	}
	list = append(list, Entry{
		DB:       name,
		CreateAt: time.Now().UTC().Format(time.RFC3339),
	})
	return r.save(list)
}

func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	list, err := r.load()
	if err != nil {
		return err
	}
	out := make([]Entry, 0, len(list))
	found := false
	for _, e := range list {
		if e.DB == name {
			found = true
			continue
		}
		out = append(out, e)
	}
	if !found {
		return fmt.Errorf("db %q not registered", name)
	}
	return r.save(out)
}

func (r *Registry) Rename(oldName, newName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	list, err := r.load()
	if err != nil {
		return err
	}
	oldIdx := -1
	for i, e := range list {
		if e.DB == newName {
			return fmt.Errorf("db %q already registered", newName)
		}
		if e.DB == oldName {
			oldIdx = i
		}
	}
	if oldIdx < 0 {
		return fmt.Errorf("db %q not registered", oldName)
	}
	list[oldIdx].DB = newName
	return r.save(list)
}

func (r *Registry) save(list []Entry) error {
	return goUtils_filesystem.WriteJSON(r.path, list, true)
}
