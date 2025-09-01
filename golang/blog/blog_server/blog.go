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
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

/////////////////////// Blog specific function /////////////////////////////////

// CreateBlogPost creates a new blog post owned by the authenticated account.
// It persists the post, sets ownership in RBAC, and indexes it in Bleve.
// Note: Public method signature must not change.
func (srv *server) CreateBlogPost(ctx context.Context, rqst *blogpb.CreateBlogPostRequest) (*blogpb.CreateBlogPostResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
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
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Owner
	if err := srv.addResourceOwner(uuid, "blog", clientId, rbacpb.SubjectType_ACCOUNT); err != nil {
		slog.Error("addResourceOwner failed", "uuid", uuid, "author", clientId, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Index
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		slog.Error("getIndex failed", "path", rqst.IndexPath, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := index.Index(uuid, blogPost); err != nil {
		slog.Error("index.Index failed", "uuid", uuid, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Store a trimmed version internally (no full Text)
	text := blogPost.Text
	blogPost.Text = ""
	if raw, err := protojson.Marshal(blogPost); err == nil {
		if err := index.SetInternal([]byte(uuid), raw); err != nil {
			slog.Error("index.SetInternal failed", "uuid", uuid, "err", err)
			// not fatal for creation, but return error for consistency
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		slog.Error("protojson.Marshal failed", "uuid", uuid, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		slog.Error("getIndex failed", "path", rqst.IndexPath, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err := index.Index(rqst.BlogPost.Uuid, rqst.BlogPost); err != nil {
		slog.Error("index.Index failed", "uuid", rqst.BlogPost.Uuid, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Store trimmed copy internally
	keep := rqst.BlogPost.Text
	rqst.BlogPost.Text = ""
	if raw, err := protojson.Marshal(rqst.BlogPost); err == nil {
		if err := index.SetInternal([]byte(rqst.BlogPost.Uuid), raw); err != nil {
			slog.Error("index.SetInternal failed", "uuid", rqst.BlogPost.Uuid, "err", err)
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		slog.Error("protojson.Marshal failed", "uuid", rqst.BlogPost.Uuid, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
			return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
			return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
			return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		slog.Error("getIndex failed", "path", rqst.IndexPath, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err := index.Delete(rqst.Uuid); err != nil {
		slog.Error("index.Delete failed", "uuid", rqst.Uuid, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := index.DeleteInternal([]byte(rqst.Uuid)); err != nil {
		slog.Error("index.DeleteInternal failed", "uuid", rqst.Uuid, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Validate emoji JSON (content not used further here, but format is verified)
	tmp := make(map[string]interface{}, 0)
	if err := json.Unmarshal([]byte(rqst.Emoji.Emoji), &tmp); err != nil {
		slog.Error("invalid emoji JSON", "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rqst.Emoji.CreationTime = time.Now().Unix()

	blog, err := srv.getBlogPost(rqst.Uuid)
	if err != nil {
		slog.Error("getBlogPost failed", "uuid", rqst.Uuid, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if comment.Emotions == nil {
			comment.Emotions = make([]*blogpb.Emoji, 0)
		}
		comment.Emotions = append(comment.Emotions, rqst.Emoji)
	}

	if err := srv.saveBlogPost(blog.Author, blog); err != nil {
		slog.Error("saveBlogPost failed after AddEmoji", "uuid", blog.Uuid, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	blog, err := srv.getBlogPost(rqst.Uuid)
	if err != nil {
		slog.Error("getBlogPost failed", "uuid", rqst.Uuid, "err", err)
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rqst.Comment.CreationTime = time.Now().Unix()
	rqst.Comment.Uuid = Utility.RandomUUID()

	parentUuid := rqst.Comment.Parent
	if parentUuid != rqst.Uuid {
		if len(parentUuid) > 0 {
			if blog.Comments == nil {
				err := errors.New("no parent comment comment found")
				return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			parentComment, err := srv.getBlogComment(parentUuid, blog)
			if err != nil {
				slog.Error("getBlogComment failed", "parent", parentUuid, "err", err)
				return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
