package main

import (
    "context"
    "encoding/json"
    "path/filepath"
    "testing"

    "github.com/globulario/services/golang/blog/blogpb"
    "github.com/globulario/services/golang/rbac/rbacpb"
    "google.golang.org/grpc/metadata"
    "google.golang.org/protobuf/encoding/protojson"
)

type fakeStore struct{
    data map[string][]byte
}

func newFakeStore() *fakeStore { return &fakeStore{data: map[string][]byte{}} }

func (s *fakeStore) Open(optionsStr string) error { return nil }
func (s *fakeStore) Close() error { return nil }
func (s *fakeStore) SetItem(key string, val []byte) error { s.data[key] = val; return nil }
func (s *fakeStore) GetItem(key string) ([]byte, error) {
    if v, ok := s.data[key]; ok {
        return v, nil
    }
    return nil, errNotFound
}
func (s *fakeStore) RemoveItem(key string) error { delete(s.data, key); return nil }
func (s *fakeStore) Clear() error { s.data = map[string][]byte{}; return nil }
func (s *fakeStore) Drop() error { s.data = map[string][]byte{}; return nil }
func (s *fakeStore) GetAllKeys() ([]string, error) { keys := make([]string, 0, len(s.data)); for k := range s.data { keys = append(keys, k) }; return keys, nil }

type recordingRbac struct{ calls int; lastToken, lastPath, lastSubject, lastType string; lastSubjectType rbacpb.SubjectType }
func (r *recordingRbac) AddResourceOwner(token, path, subject, resourceType string, subjectType rbacpb.SubjectType) error {
    r.calls++
    r.lastToken, r.lastPath, r.lastSubject, r.lastType, r.lastSubjectType = token, path, subject, resourceType, subjectType
    return nil
}

var errNotFound = &notFoundErr{}
type notFoundErr struct{}
func (e *notFoundErr) Error() string { return "not found" }

func bootstrapCtx(t *testing.T, clientID string) context.Context {
    return metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
        "token", "internal-bootstrap",
        "x-globular-internal", "true",
        "client-id", clientID,
    ))
}

func TestCreateBlogPostStoresAndIndexes(t *testing.T) {
    srv, rbac := newTestServer()

    idxDir := t.TempDir()

    ctx := bootstrapCtx(t, "user-1")
    resp, err := srv.CreateBlogPost(ctx, &blogpb.CreateBlogPostRequest{IndexPath: filepath.Join(idxDir, "idx"), Text: "hello", Title: "t"})
    if err != nil {
        t.Fatalf("CreateBlogPost error = %v", err)
    }
    if resp.GetBlogPost() == nil || resp.GetBlogPost().Uuid == "" {
        t.Fatalf("BlogPost missing in response")
    }

    // Ensure stored blob exists
    data, err := srv.store.GetItem(resp.BlogPost.Uuid)
    if err != nil {
        t.Fatalf("stored blog missing: %v", err)
    }
    stored := &blogpb.BlogPost{}
    if err := protojson.Unmarshal(data, stored); err != nil {
        t.Fatalf("unmarshal stored blog: %v", err)
    }

    if stored.Title != "t" {
        t.Errorf("stored.Title = %q, want t", stored.Title)
    }

    if rbac.calls != 1 {
        t.Fatalf("expected AddResourceOwner to be called once, got %d", rbac.calls)
    }
}

func TestAddCommentAttachesToPost(t *testing.T) {
    srv, _ := newTestServer()

    // seed a post
    ctx := bootstrapCtx(t, "user-1")
    resp, err := srv.CreateBlogPost(ctx, &blogpb.CreateBlogPostRequest{IndexPath: filepath.Join(t.TempDir(), "idx"), Text: "body", Title: "t"})
    if err != nil {
        t.Fatalf("CreateBlogPost failed: %v", err)
    }

    cctx := bootstrapCtx(t, "user-1")
    addResp, err := srv.AddComment(cctx, &blogpb.AddCommentRequest{
        Uuid: resp.BlogPost.Uuid,
        Comment: &blogpb.Comment{Parent: resp.BlogPost.Uuid, AccountId: "user-1", Text: "hi"},
    })
    if err != nil {
        t.Fatalf("AddComment error = %v", err)
    }
    if addResp.Comment == nil || addResp.Comment.Uuid == "" {
        t.Fatalf("AddComment did not return comment")
    }

    // Ensure stored post has comment
    data, err := srv.store.GetItem(resp.BlogPost.Uuid)
    if err != nil {
        t.Fatalf("post missing: %v", err)
    }
    stored := &blogpb.BlogPost{}
    if err := protojson.Unmarshal(data, stored); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if len(stored.Comments) != 1 {
        t.Fatalf("expected 1 comment, got %d", len(stored.Comments))
    }
}

func TestDeleteBlogPostRemovesFromAuthorIndex(t *testing.T) {
    srv, _ := newTestServer()
    fs := srv.store.(*fakeStore)

    ctx := bootstrapCtx(t, "user-1")
    resp, err := srv.CreateBlogPost(ctx, &blogpb.CreateBlogPostRequest{IndexPath: filepath.Join(t.TempDir(), "idx"), Text: "body", Title: "t"})
    if err != nil {
        t.Fatalf("CreateBlogPost failed: %v", err)
    }

    // Delete
    dctx := bootstrapCtx(t, "user-1")
    if _, err := srv.DeleteBlogPost(dctx, &blogpb.DeleteBlogPostRequest{Uuid: resp.BlogPost.Uuid, IndexPath: filepath.Join(t.TempDir(), "idx2")}); err != nil {
        t.Fatalf("DeleteBlogPost error = %v", err)
    }

    if _, err := fs.GetItem(resp.BlogPost.Uuid); err == nil {
        t.Fatalf("blog still exists after deletion")
    }

    // author index should be empty
    if idsBytes, err := fs.GetItem("user-1"); err == nil {
        var ids []string
        if err := json.Unmarshal(idsBytes, &ids); err != nil {
            t.Fatalf("unmarshal ids: %v", err)
        }
        if len(ids) != 0 {
            t.Fatalf("expected author index empty, got %d", len(ids))
        }
    }
}

// newTestServer returns a server wired with in-memory fakes for store and RBAC.
func newTestServer() (*server, *recordingRbac) {
    srv := initializeServerDefaults()
    fs := newFakeStore()
    srv.store = fs
    r := &recordingRbac{}
    srv.rbacClientFactory = func(address string) (rbacOwnerClient, error) { return r, nil }
    return srv, r
}
