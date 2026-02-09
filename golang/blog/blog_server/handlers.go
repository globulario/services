package main

import (
    "context"
    "encoding/json"
    "errors"
    "log/slog"
    "sort"
    "time"

    "github.com/blevesearch/bleve/v2"
    "github.com/globulario/services/golang/blog/blogpb"
    "github.com/globulario/services/golang/event/event_client"
    "github.com/globulario/services/golang/event/eventpb"
    "github.com/globulario/services/golang/globular_client"
    "github.com/globulario/services/golang/rbac/rbac_client"
    "github.com/globulario/services/golang/rbac/rbacpb"
    "github.com/globulario/services/golang/security"
    Utility "github.com/globulario/utility"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/encoding/protojson"
)

// Interfaces to allow dependency injection in tests.
type eventSubscriber interface {
    Subscribe(evt string, consumer string, listener func(evt *eventpb.Event)) error
}

type eventClientFactory func() (eventSubscriber, error)

type rbacOwnerClient interface {
    AddResourceOwner(token, path, subject, resourceType string, subjectType rbacpb.SubjectType) error
}

type rbacClientFactory func(address string) (rbacOwnerClient, error)

// -----------------------------------------------------------------------------
// Event helpers
// -----------------------------------------------------------------------------

func defaultEventClient(address string) (eventSubscriber, error) {
    Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
    client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
    if err != nil {
        return nil, err
    }
    return client.(*event_client.Event_Client), nil
}

func (srv *server) getEventClient() (eventSubscriber, error) {
    if srv.eventClientFactory != nil {
        return srv.eventClientFactory()
    }
    return defaultEventClient(srv.Address)
}

func (srv *server) subscribe(evt string, listener func(evt *eventpb.Event)) error {
    eventClient, err := srv.getEventClient()
    if err != nil {
        return err
    }
    return eventClient.Subscribe(evt, srv.Name, listener)
}

// -----------------------------------------------------------------------------
// RBAC helpers
// -----------------------------------------------------------------------------

func defaultRbacClient(address string) (*rbac_client.Rbac_Client, error) {
    Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
    client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
    if err != nil {
        return nil, err
    }
    return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) getRbacClient() (rbacOwnerClient, error) {
    if srv.rbacClientFactory != nil {
        return srv.rbacClientFactory(srv.Address)
    }
    return defaultRbacClient(srv.Address)
}

func (srv *server) addResourceOwner(token, path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
    client, err := srv.getRbacClient()
    if err != nil {
        return err
    }
    return client.AddResourceOwner(token, path, subject, resourceType, subjectType)
}

// -----------------------------------------------------------------------------
// Bleve helpers
// -----------------------------------------------------------------------------

// getIndex opens or creates a Bleve index at the given path and caches it.
func (srv *server) getIndex(path string) (bleve.Index, error) {
    if srv.indexs == nil {
        srv.indexs = make(map[string]bleve.Index)
    }
    if srv.indexs[path] == nil {
        index, err := bleve.Open(path)
        if err != nil {
            // Create a new index if opening failed.
            mapping := bleve.NewIndexMapping()
            index, err = bleve.New(path, mapping)
            if err != nil {
                logger.Error("create bleve index failed", "path", path, "err", err)
                return nil, err
            }
            logger.Info("created new bleve index", "path", path)
        } else {
            logger.Info("opened existing bleve index", "path", path)
        }
        srv.indexs[path] = index
    }
    return srv.indexs[path], nil
}

// -----------------------------------------------------------------------------
// Blog helpers
// -----------------------------------------------------------------------------

func (srv *server) deleteAccountListener(evt *eventpb.Event) {
    accountId := string(evt.Data)
    blogs, err := srv.getBlogPostByAuthor(accountId)
    if err != nil {
        logger.Error("get blogs by author failed", "author", accountId, "err", err)
        return
    }
    for i := 0; i < len(blogs); i++ {
        if err := srv.deleteBlogPost(accountId, blogs[i].Uuid); err != nil {
            logger.Error("delete blog post failed", "author", accountId, "uuid", blogs[i].Uuid, "err", err)
        } else {
            logger.Info("deleted blog post for removed account", "author", accountId, "uuid", blogs[i].Uuid)
        }
    }
}

// getBlogPost returns the blog post with the given uuid.
func (srv *server) getBlogPost(uuid string) (*blogpb.BlogPost, error) {
    blog := new(blogpb.BlogPost)
    jsonStr, err := srv.store.GetItem(uuid)
    if err != nil {
        return nil, err
    }
    if err := protojson.Unmarshal(jsonStr, blog); err != nil {
        return nil, err
    }
    return blog, nil
}

// getBlogPostByAuthor returns all blog posts authored by the given account id.
func (srv *server) getBlogPostByAuthor(author string) ([]*blogpb.BlogPost, error) {
    blogPosts := make([]*blogpb.BlogPost, 0)

    blogsBytes, err := srv.store.GetItem(author)
    ids := make([]string, 0)
    if err == nil {
        if err := json.Unmarshal(blogsBytes, &ids); err != nil {
            return nil, err
        }
    }

    for i := 0; i < len(ids); i++ {
        jsonStr, err := srv.store.GetItem(ids[i])
        if err != nil {
            continue
        }
        instance := new(blogpb.BlogPost)
        if err := protojson.Unmarshal(jsonStr, instance); err == nil {
            blogPosts = append(blogPosts, instance)
        }
    }

    return blogPosts, nil
}

// getSubComment searches recursively for a sub-comment inside a comment tree.
func (srv *server) getSubComment(uuid string, comment *blogpb.Comment) (*blogpb.Comment, error) {
    if comment.Comments == nil {
        return nil, errors.New("no answer was found for that comment")
    }
    for i := 0; i < len(comment.Comments); i++ {
        c := comment.Comments[i]
        if uuid == c.Uuid {
            return c, nil
        }
        if c.Comments != nil {
            if found, err := srv.getSubComment(uuid, c); err == nil && found != nil {
                return found, nil
            }
        }
    }
    return nil, errors.New("no answer was found for that comment")
}

// getBlogComment finds a comment by uuid within a blog post (searching answers recursively).
func (srv *server) getBlogComment(parentUuid string, blog *blogpb.BlogPost) (*blogpb.Comment, error) {
    for i := 0; i < len(blog.Comments); i++ {
        c := blog.Comments[i]
        if c.Uuid == parentUuid {
            return c, nil
        }
        if found, err := srv.getSubComment(parentUuid, c); err == nil && found != nil {
            return found, nil
        }
    }
    return nil, errors.New("no comment was found for that blog")
}

// deleteBlogPost deletes a blog post if requested by its author.
func (srv *server) deleteBlogPost(author, uuid string) error {
    blog, err := srv.getBlogPost(uuid)
    if err != nil {
        return err
    }
    if author != blog.Author {
        return errors.New("only blog author can delete it blog")
    }

    // Remove from author index list.
    blogsBytes, err := srv.store.GetItem(blog.Author)
    ids := make([]string, 0)
    if err == nil {
        if err := json.Unmarshal(blogsBytes, &ids); err != nil {
            return err
        }
    }
    ids = Utility.RemoveString(ids, uuid)

    // Save updated list.
    idsJSON, err := Utility.ToJson(ids)
    if err != nil {
        return err
    }
    if err := srv.store.SetItem(blog.Author, []byte(idsJSON)); err != nil {
        return err
    }

    // Delete the post object.
    return srv.store.RemoveItem(uuid)
}

// saveBlogPost persists a blog post and maintains the author's index list.
func (srv *server) saveBlogPost(author string, blogPost *blogpb.BlogPost) error {
    blogPost.Domain = srv.Domain
    blogPost.Mac = srv.Mac

    jsonStr, err := protojson.Marshal(blogPost)
    if err != nil {
        return err
    }
    if err := srv.store.SetItem(blogPost.Uuid, []byte(jsonStr)); err != nil {
        return err
    }

    // Update author index.
    blogsBytes, err := srv.store.GetItem(author)
    blogs := make([]string, 0)
    if err == nil {
        _ = json.Unmarshal(blogsBytes, &blogs)
    }
    if !Utility.Contains(blogs, blogPost.Uuid) {
        blogs = append(blogs, blogPost.Uuid)
    }
    blogsJSON, err := Utility.ToJson(blogs)
    if err != nil {
        return err
    }
    return srv.store.SetItem(author, []byte(blogsJSON))
}

// -----------------------------------------------------------------------------
// RPC Handlers
// -----------------------------------------------------------------------------

// CreateBlogPost creates a new blog post owned by the authenticated account.
// It persists the post, sets ownership in RBAC, and indexes it in Bleve.
// Note: Public method signature must not change.
func (srv *server) CreateBlogPost(ctx context.Context, rqst *blogpb.CreateBlogPostRequest) (*blogpb.CreateBlogPostResponse, error) {
    clientId, token, err := security.GetClientId(ctx)
    if err != nil {
        slog.Error("GetClientId failed", "err", err)
        return nil, err
    }

    uuid := Utility.RandomUUID()
    if len(rqst.Language) == 0 {
        rqst.Language = "en"
    }

    blogPost := &blogpb.BlogPost{
        Uuid:         uuid,
        Author:       clientId,
        Keywords:     rqst.Keywords,
        CreationTime: time.Now().Unix(),
        Language:     rqst.Language,
        Text:         rqst.Text,
        Title:        rqst.Title,
        Subtitle:     rqst.Subtitle,
        Thumbnail:    rqst.Thumbnail,
        Status:       blogpb.BogPostStatus_DRAFT,
    }

    // Save
    if err := srv.saveBlogPost(clientId, blogPost); err != nil {
        slog.Error("saveBlogPost failed", "uuid", uuid, "author", clientId, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    // Owner
    if err := srv.addResourceOwner(token, uuid, clientId, "blog", rbacpb.SubjectType_ACCOUNT); err != nil {
        slog.Error("addResourceOwner failed", "uuid", uuid, "author", clientId, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    // Index
    index, err := srv.getIndex(rqst.IndexPath)
    if err != nil {
        slog.Error("getIndex failed", "path", rqst.IndexPath, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }
    if err := index.Index(uuid, blogPost); err != nil {
        slog.Error("index.Index failed", "uuid", uuid, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    // Store a trimmed version internally (no full Text)
    text := blogPost.Text
    blogPost.Text = ""
    if raw, err := protojson.Marshal(blogPost); err == nil {
        if err := index.SetInternal([]byte(uuid), raw); err != nil {
            slog.Error("index.SetInternal failed", "uuid", uuid, "err", err)
            // not fatal for creation, but return error for consistency
            return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
        }
    } else {
        slog.Error("protojson.Marshal failed", "uuid", uuid, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }
    blogPost.Text = text

    slog.Info("blog post created", "uuid", uuid, "author", clientId)
    return &blogpb.CreateBlogPostResponse{BlogPost: blogPost}, nil
}

// SaveBlogPost updates an existing blog post (owned by the current account) and re-indexes it.
// The full object is persisted; the index stores a trimmed internal copy without Text.
// Note: Public method signature must not change.
func (srv *server) SaveBlogPost(ctx context.Context, rqst *blogpb.SaveBlogPostRequest) (*blogpb.SaveBlogPostResponse, error) {
    clientId, _, err := security.GetClientId(ctx)
    if err != nil {
        slog.Error("GetClientId failed", "err", err)
        return nil, err
    }

    if err := srv.saveBlogPost(clientId, rqst.BlogPost); err != nil {
        slog.Error("saveBlogPost failed", "uuid", rqst.BlogPost.Uuid, "author", clientId, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    index, err := srv.getIndex(rqst.IndexPath)
    if err != nil {
        slog.Error("getIndex failed", "path", rqst.IndexPath, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    if err := index.Index(rqst.BlogPost.Uuid, rqst.BlogPost); err != nil {
        slog.Error("index.Index failed", "uuid", rqst.BlogPost.Uuid, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    // Store trimmed copy internally
    keep := rqst.BlogPost.Text
    rqst.BlogPost.Text = ""
    if raw, err := protojson.Marshal(rqst.BlogPost); err == nil {
        if err := index.SetInternal([]byte(rqst.BlogPost.Uuid), raw); err != nil {
            slog.Error("index.SetInternal failed", "uuid", rqst.BlogPost.Uuid, "err", err)
            return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
        }
    } else {
        slog.Error("protojson.Marshal failed", "uuid", rqst.BlogPost.Uuid, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }
    rqst.BlogPost.Text = keep

    slog.Info("blog post saved", "uuid", rqst.BlogPost.Uuid, "author", clientId)
    return &blogpb.SaveBlogPostResponse{}, nil
}

// GetBlogPostsByAuthors streams blog posts for the provided author IDs.
// Posts are sorted by CreationTime (ascending) and up to rqst.Max items are sent.
// Note: Public method signature must not change.
func (srv *server) GetBlogPostsByAuthors(rqst *blogpb.GetBlogPostsByAuthorsRequest, stream blogpb.BlogService_GetBlogPostsByAuthorsServer) error {
    blogs := make([]*blogpb.BlogPost, 0)
    for i := 0; i < len(rqst.Authors); i++ {
        list, err := srv.getBlogPostByAuthor(rqst.Authors[i])
        if err == nil {
            blogs = append(blogs, list...)
        } else {
            slog.Warn("getBlogPostByAuthor failed", "author", rqst.Authors[i], "err", err)
        }
    }

    if len(blogs) == 0 {
        return errors.New("no blog founds")
    }

    if len(blogs) > 1 {
        sort.Slice(blogs, func(i, j int) bool {
            return blogs[i].CreationTime < blogs[j].CreationTime
        })
    }

    max := int(rqst.Max)
    for i := 0; i < max && i < len(blogs); i++ {
        if err := stream.Send(&blogpb.GetBlogPostsByAuthorsResponse{BlogPost: blogs[i]}); err != nil {
            slog.Error("stream.Send failed", "idx", i, "uuid", blogs[i].Uuid, "err", err)
            return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
        }
    }

    return nil
}

// GetBlogPosts streams blog posts by explicit UUIDs.
// Missing or invalid UUIDs are logged and skipped.
// Note: Public method signature must not change.
func (srv *server) GetBlogPosts(rqst *blogpb.GetBlogPostsRequest, stream blogpb.BlogService_GetBlogPostsServer) error {
    for i := 0; i < len(rqst.Uuids); i++ {
        id := rqst.Uuids[i]
        data, err := srv.store.GetItem(id)
        if err != nil {
            slog.Warn("store.GetItem failed", "uuid", id, "err", err)
            continue
        }
        b := new(blogpb.BlogPost)
        if err := protojson.Unmarshal(data, b); err != nil {
            slog.Warn("protojson.Unmarshal failed", "uuid", id, "err", err)
            continue
        }
        if err := stream.Send(&blogpb.GetBlogPostsResponse{BlogPost: b}); err != nil {
            slog.Error("stream.Send failed", "uuid", id, "err", err)
            return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
        }
    }
    return nil
}

// SearchBlogPosts performs a Bleve search over posts and streams:
// 1) a summary, 2) each hit with snippets and trimmed Blog, and 3) the facets.
// Note: Public method signature must not change.
func (srv *server) SearchBlogPosts(rqst *blogpb.SearchBlogPostsRequest, stream blogpb.BlogService_SearchBlogPostsServer) error {
    index, err := srv.getIndex(rqst.IndexPath)
    if err != nil {
        slog.Error("getIndex failed", "path", rqst.IndexPath, "err", err)
        return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    query := bleve.NewQueryStringQuery(rqst.Query)
    request := bleve.NewSearchRequest(query)
    request.Size = int(rqst.Size)
    request.From = int(rqst.Offset)
    if request.Size == 0 {
        request.Size = 50
    }

    // Facets
    tags := bleve.NewFacetRequest("Keywords", int(rqst.Size))
    request.AddFacet("Keywords", tags)

    request.Highlight = bleve.NewHighlightWithStyle("html")
    request.Fields = rqst.Fields

    result, err := index.Search(request)
    if err != nil {
        slog.Error("index.Search failed", "query", rqst.Query, "err", err)
        return err
    }

    // Summary
    summary := &blogpb.SearchSummary{
        Query: rqst.Query,
        Took:  result.Took.Milliseconds(),
        Total: result.Total,
    }
    if err := stream.Send(&blogpb.SearchBlogPostsResponse{Result: &blogpb.SearchBlogPostsResponse_Summary{Summary: summary}}); err != nil {
        slog.Error("stream.Send summary failed", "err", err)
        return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    // Hits
    for i, hit := range result.Hits {
        id := hit.ID
        h := &blogpb.SearchHit{
            Score:    hit.Score,
            Index:    int32(i),
            Snippets: make([]*blogpb.Snippet, 0),
        }

        for fragmentField, fragments := range hit.Fragments {
            snippet := &blogpb.Snippet{
                Field:     fragmentField,
                Fragments: make([]string, 0, len(fragments)),
            }
            for _, fragment := range fragments {
                snippet.Fragments = append(snippet.Fragments, fragment)
            }
            h.Snippets = append(h.Snippets, snippet)
        }

        if raw, err := index.GetInternal([]byte(id)); err == nil {
            post := new(blogpb.BlogPost)
            if err := protojson.Unmarshal(raw, post); err == nil {
                h.Blog = post
            } else {
                slog.Warn("protojson.Unmarshal internal failed", "uuid", id, "err", err)
            }
        } else {
            slog.Warn("index.GetInternal failed", "uuid", id, "err", err)
        }

        if err := stream.Send(&blogpb.SearchBlogPostsResponse{Result: &blogpb.SearchBlogPostsResponse_Hit{Hit: h}}); err != nil {
            slog.Error("stream.Send hit failed", "idx", i, "uuid", id, "err", err)
            return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
        }
    }

    // Facets
    facets := &blogpb.SearchFacets{Facets: make([]*blogpb.SearchFacet, 0, len(result.Facets))}
    for _, f := range result.Facets {
        facet := &blogpb.SearchFacet{
            Field: f.Field,
            Total: int32(f.Total),
            Other: int32(f.Other),
            Terms: make([]*blogpb.SearchFacetTerm, 0, len(f.Terms.Terms())),
        }
        for _, t := range f.Terms.Terms() {
            facet.Terms = append(facet.Terms, &blogpb.SearchFacetTerm{
                Term:  t.Term,
                Count: int32(t.Count),
            })
        }
        facets.Facets = append(facets.Facets, facet)
    }

    if err := stream.Send(&blogpb.SearchBlogPostsResponse{Result: &blogpb.SearchBlogPostsResponse_Facets{Facets: facets}}); err != nil {
        slog.Error("stream.Send facets failed", "err", err)
        return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    return nil
}

// DeleteBlogPost deletes a blog post (must be owned by the requester) and removes it from the index.
// Note: Public method signature must not change.
func (srv *server) DeleteBlogPost(ctx context.Context, rqst *blogpb.DeleteBlogPostRequest) (*blogpb.DeleteBlogPostResponse, error) {
    clientId, _, err := security.GetClientId(ctx)
    if err != nil {
        slog.Error("GetClientId failed", "err", err)
        return nil, err
    }

    if err := srv.deleteBlogPost(clientId, rqst.Uuid); err != nil {
        slog.Error("deleteBlogPost failed", "uuid", rqst.Uuid, "author", clientId, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    index, err := srv.getIndex(rqst.IndexPath)
    if err != nil {
        slog.Error("getIndex failed", "path", rqst.IndexPath, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    if err := index.Delete(rqst.Uuid); err != nil {
        slog.Error("index.Delete failed", "uuid", rqst.Uuid, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }
    if err := index.DeleteInternal([]byte(rqst.Uuid)); err != nil {
        slog.Error("index.DeleteInternal failed", "uuid", rqst.Uuid, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    slog.Info("blog post deleted", "uuid", rqst.Uuid, "author", clientId)
    return &blogpb.DeleteBlogPostResponse{}, nil
}

// AddEmoji adds an emoji on a blog or a specific comment (parent).
// The emoji payload (Emoji.Emoji) must be a JSON string; it is parsed to validate structure.
// Note: Public method signature must not change.
func (srv *server) AddEmoji(ctx context.Context, rqst *blogpb.AddEmojiRequest) (*blogpb.AddEmojiResponse, error) {
    clientId, _, err := security.GetClientId(ctx)
    if err != nil {
        slog.Error("GetClientId failed", "err", err)
        return nil, err
    }

    if rqst.Emoji.AccountId != clientId {
        err := errors.New("you can't comment for another account")
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    // Validate emoji JSON (content not used further here, but format is verified)
    tmp := make(map[string]interface{}, 0)
    if err := json.Unmarshal([]byte(rqst.Emoji.Emoji), &tmp); err != nil {
        slog.Error("invalid emoji JSON", "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    rqst.Emoji.CreationTime = time.Now().Unix()

    blog, err := srv.getBlogPost(rqst.Uuid)
    if err != nil {
        slog.Error("getBlogPost failed", "uuid", rqst.Uuid, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    if blog.Uuid == rqst.Emoji.Parent {
        if blog.Emotions == nil {
            blog.Emotions = make([]*blogpb.Emoji, 0)
        }
        blog.Emotions = append(blog.Emotions, rqst.Emoji)
    } else {
        comment, err := srv.getBlogComment(rqst.Emoji.Parent, blog)
        if err != nil {
            slog.Error("getBlogComment failed", "parent", rqst.Emoji.Parent, "err", err)
            return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
        }
        if comment.Emotions == nil {
            comment.Emotions = make([]*blogpb.Emoji, 0)
        }
        comment.Emotions = append(comment.Emotions, rqst.Emoji)
    }

    if err := srv.saveBlogPost(blog.Author, blog); err != nil {
        slog.Error("saveBlogPost failed after AddEmoji", "uuid", blog.Uuid, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    slog.Info("emoji added", "post_uuid", rqst.Uuid, "parent", rqst.Emoji.Parent, "by", clientId)
    return &blogpb.AddEmojiResponse{Emoji: rqst.Emoji}, nil
}

// RemoveEmoji removes an emoji from a post or comment (TODO/placeholder).
// Note: Public method signature must not change.
func (srv *server) RemoveEmoji(ctx context.Context, rqst *blogpb.RemoveEmojiRequest) (*blogpb.RemoveEmojiResponse, error) {
    // Not implemented yet — kept to preserve API surface.
    slog.Warn("RemoveEmoji not implemented")
    return nil, nil
}

// AddComment adds a new comment (or reply to an existing comment) on a blog post.
// When Parent equals the post UUID, the comment is attached to the post; otherwise
// it is attached as a reply under the parent comment.
// Note: Public method signature must not change.
func (srv *server) AddComment(ctx context.Context, rqst *blogpb.AddCommentRequest) (*blogpb.AddCommentResponse, error) {
    clientId, _, err := security.GetClientId(ctx)
    if err != nil {
        slog.Error("GetClientId failed", "err", err)
        return nil, err
    }

    if rqst.Comment.AccountId != clientId {
        err := errors.New("you can't comment for another account")
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    blog, err := srv.getBlogPost(rqst.Uuid)
    if err != nil {
        slog.Error("getBlogPost failed", "uuid", rqst.Uuid, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    rqst.Comment.CreationTime = time.Now().Unix()
    rqst.Comment.Uuid = Utility.RandomUUID()

    parentUuid := rqst.Comment.Parent
    if parentUuid != rqst.Uuid {
        if len(parentUuid) > 0 {
            if blog.Comments == nil {
                err := errors.New("no parent comment comment found")
                return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
            }
            parentComment, err := srv.getBlogComment(parentUuid, blog)
            if err != nil {
                slog.Error("getBlogComment failed", "parent", parentUuid, "err", err)
                return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
            }
            parentComment.Comments = append(parentComment.Comments, rqst.Comment)
        }
    } else {
        if blog.Comments == nil {
            blog.Comments = make([]*blogpb.Comment, 0)
        }
        blog.Comments = append(blog.Comments, rqst.Comment)
    }

    if err := srv.saveBlogPost(clientId, blog); err != nil {
        slog.Error("saveBlogPost failed after AddComment", "uuid", blog.Uuid, "err", err)
        return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
    }

    slog.Info("comment added", "post_uuid", rqst.Uuid, "comment_uuid", rqst.Comment.Uuid, "by", clientId)
    return &blogpb.AddCommentResponse{Comment: rqst.Comment}, nil
}

// RemoveComment removes a comment from a post or a reply thread (TODO/placeholder).
// Note: Public method signature must not change.
func (srv *server) RemoveComment(ctx context.Context, rqst *blogpb.RemoveCommentRequest) (*blogpb.RemoveCommentResponse, error) {
    // Not implemented yet — kept to preserve API surface.
    slog.Warn("RemoveComment not implemented")
    return nil, nil
}
