package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/blog/blogpb"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/metadata"

	//"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/////////////////////// Blog specific function /////////////////////////////////

// One request followed by one response
func (svr *server) CreateBlogPost(ctx context.Context, rqst *blogpb.CreateBlogPostRequest) (*blogpb.CreateBlogPostResponse, error) {

	// So here I will create a new blog from the infromation sent by the user.
	var clientId string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			clientId, _, _, _, _, err = security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		} else {
			errors.New("no token was given")
		}
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
		Thumbnail:     rqst.Thumbnail,
		Status:       blogpb.BogPostStatus_DRAFT,
	}

	// Save the blog.
	err = svr.saveBlogPost(clientId, blogPost)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will set it in the rbac as ressource owner...
	permissions := &rbacpb.Permissions{
		Allowed: []*rbacpb.Permission{},
		Denied:  []*rbacpb.Permission{},
		Owners: &rbacpb.Permission{
			Name:          "owner", // The name is informative in that particular case.
			Applications:  []string{},
			Accounts:      []string{clientId},
			Groups:        []string{},
			Peers:         []string{},
			Organizations: []string{},
		},
	}

	// Set the owner of the conversation.
	err = svr.setResourcePermissions(uuid, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// TODO send publish event also..
	return &blogpb.CreateBlogPostResponse{BlogPost: blogPost}, nil
}

// Update a blog post...
func (svr *server) SaveBlogPost(ctx context.Context, rqst *blogpb.SaveBlogPostRequest) (*blogpb.SaveBlogPostResponse, error) {
	var clientId string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			clientId, _, _, _, _, err = security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		} else {
			errors.New("no token was given")
		}
	}

	// Save the blog.
	err = svr.saveBlogPost(clientId, rqst.BlogPost)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// TODO send publish event also..
	return &blogpb.SaveBlogPostResponse{}, nil
}

// Retreive Blog Post by author
func (svr *server) GetBlogPostsByAuthor(ctx context.Context, rqst *blogpb.GetBlogPostsByAuthorRequest) (*blogpb.GetBlogPostsByAuthorResponse, error) {

	// So here I will get the list of blogpost for a given author...
	blogs, err := svr.getBlogPostByAuthor(rqst.Author)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &blogpb.GetBlogPostsByAuthorResponse{
		Posts: blogs,
	}, nil
}

// Search blog by keyword's or text find in the post...
func (svr *server) SearchBlogPosts(ctx context.Context, rqst *blogpb.SearchBlogsPostRequest) (*blogpb.SearchBlogsPostResponse, error) {
	return nil, nil
}

// Delete a blog.
func (svr *server) DeleteBlogPost(ctx context.Context, rqst *blogpb.DeleteBlogPostRequest) (*blogpb.DeleteBlogPostResponse, error) {

	var clientId string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			clientId, _, _, _, _, err = security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		} else {
			errors.New("no token was given")
		}
	}

	err = svr.deleteBlogPost(clientId, rqst.Uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &blogpb.DeleteBlogPostResponse{}, nil
}

// Like a post or comment
func (svr *server) AddLike(ctx context.Context, rqst *blogpb.AddLikeRequest) (*blogpb.AddLikeResponse, error) {
	return nil, nil
}

// Remove like from a post or comment.
func (svr *server) RemoveLike(ctx context.Context, rqst *blogpb.RemoveLikeRequest) (*blogpb.RemoveLikeResponse, error) {
	return nil, nil
}

// Dislike a post or comment
func (svr *server) AddDislike(ctx context.Context, rqst *blogpb.AddLikeRequest) (*blogpb.AddLikeResponse, error) {
	return nil, nil
}

// Remove dislike from a post or comment.
func (svr *server) RemoveDislike(ctx context.Context, rqst *blogpb.RemoveDislikeRequest) (*blogpb.RemoveDislikeResponse, error) {
	return nil, nil
}

// Comment a post or comment
func (svr *server) AddComment(ctx context.Context, rqst *blogpb.AddCommentRequest) (*blogpb.AddCommentResponse, error) {
	return nil, nil
}

// Remove comment from a post or comment.
func (svr *server) RemoveComment(ctx context.Context, rqst *blogpb.RemoveCommentRequest) (*blogpb.RemoveCommentResponse, error) {
	return nil, nil
}
