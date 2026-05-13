package main

import (
	"errors"
	"testing"
)

type fakeStore struct {
	getErr error
}

func (f *fakeStore) Open(string) error            { return nil }
func (f *fakeStore) Close() error                 { return nil }
func (f *fakeStore) SetItem(string, []byte) error { return nil }
func (f *fakeStore) GetItem(string) ([]byte, error) {
	return nil, f.getErr
}
func (f *fakeStore) RemoveItem(string) error { return nil }
func (f *fakeStore) Clear() error            { return nil }
func (f *fakeStore) Drop() error             { return nil }
func (f *fakeStore) GetAllKeys() ([]string, error) {
	return nil, nil
}

func TestEnsureScyllaReadyForStartup_QueryFailureBlocksStartup(t *testing.T) {
	srv := &server{
		connection_is_open: true,
		store:              &fakeStore{getErr: errors.New("cql unavailable")},
	}
	if err := srv.ensureScyllaReadyForStartup(); err == nil {
		t.Fatal("expected startup to be blocked when scylla query fails")
	}
}

func TestEnsureScyllaReadyForStartup_SucceedsWhenQueryWorks(t *testing.T) {
	srv := &server{
		connection_is_open: true,
		store:              &fakeStore{},
	}
	if err := srv.ensureScyllaReadyForStartup(); err != nil {
		t.Fatalf("expected startup gate success, got error: %v", err)
	}
}

