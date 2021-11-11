package main

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
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
			err := errors.New("no token was given")
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
		Thumbnail:    rqst.Thumbnail,
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
			err := errors.New("no token was given")
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
func (svr *server) GetBlogPostsByAuthors(rqst *blogpb.GetBlogPostsByAuthorsRequest, stream blogpb.BlogService_GetBlogPostsByAuthorsServer) error {

	// Retreive the list of all blogs. 
	blogs := make([]*blogpb.BlogPost, 0)
	for i:=0; i < len(rqst.Authors); i++ {
		blogs_, err := svr.getBlogPostByAuthor(rqst.Authors[i])
		if err == nil {
			blogs = append(blogs, blogs_ ...)
		}
	}

	// Now I will sort the values by creation date...
	sort.Slice(blogs, func(i, j int)bool{
		return blogs[i].CreationTime < blogs[j].CreationTime
	})

	// Finaly I will return blogs..
	max := int(rqst.Max)

	// In that case I will return the whole list.
	for i:=0; i < max && i < len(blogs); i++ {
		err := stream.Send(&blogpb.GetBlogPostsByAuthorsResponse{
			BlogPost: blogs[i],
		})
		// Return err
		if err != nil {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return nil
}

// Search blog by keyword's or text find in the post...
func (svr *server) SearchBlogPosts(rqst *blogpb.SearchBlogsPostRequest, stream blogpb.BlogService_SearchBlogPostsServer) error{
	return nil
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
			err := errors.New("no token was given")
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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

// Emoji a post or comment
func (svr *server) AddEmoji(ctx context.Context, rqst *blogpb.AddEmojiRequest) (*blogpb.AddEmojiResponse, error) {
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
			err := errors.New("no token was given")
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if rqst.Emoji.AccountId != clientId {
		err := errors.New("you can't comment for another account")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	emoji := make(map[string]interface{}, 0)

	err = json.Unmarshal([]byte(rqst.Emoji.Emoji), &emoji)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Set the time and date...
	rqst.Emoji.CreationTime = time.Now().Unix()

	// Now I will add it to it parent.
	blog, err := svr.getBlogPost(rqst.Uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if blog.Uuid == rqst.Uuid {
		// Here I will append the emoji to the blog itself...
		if blog.Emotions == nil {
			blog.Emotions = make([]*blogpb.Emoji, 0)
		}

		blog.Emotions = append(blog.Emotions, rqst.Emoji)
	} else {
		// Here I will find the comment who must contain the emoji
		comment, err := svr.getBlogComment(rqst.Emoji.Parent, blog)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// Put emotion in your comment...
		if comment.Emotions == nil {
			comment.Emotions = make([]*blogpb.Emoji, 0)
		}

		comment.Emotions = append(comment.Emotions, rqst.Emoji)
	}

	err = svr.saveBlogPost(blog.Author, blog)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &blogpb.AddEmojiResponse{Emoji: rqst.Emoji}, nil
}

// Remove like from a post or comment.
func (svr *server) RemoveEmoji(ctx context.Context, rqst *blogpb.RemoveEmojiRequest) (*blogpb.RemoveEmojiResponse, error) {
	return nil, nil
}

// Comment a post or comment
func (svr *server) AddComment(ctx context.Context, rqst *blogpb.AddCommentRequest) (*blogpb.AddCommentResponse, error) {
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
			err := errors.New("no token was given")
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if rqst.Comment.AccountId != clientId {
		err := errors.New("you can't comment for another account")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	blog, err := svr.getBlogPost(rqst.Uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I will append the comment...
	parentUuid := rqst.Comment.Parent

	// set comment variable.
	rqst.Comment.CreationTime = time.Now().Unix()
	rqst.Comment.Uuid = Utility.RandomUUID()

	// if the comment is a response to other comment then...
	if len(parentUuid) > 0 {
		// Here I need to find the comment in the blog comments...
		if blog.Comments == nil {
			err := errors.New("no parent comment comment found")
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		parentComment, err := svr.getBlogComment(parentUuid, blog)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// So here the parent comment was found...
		parentComment.Answers = append(parentComment.Answers, rqst.Comment)
	}

	// Directly comment the blog...
	if blog.Comments == nil {
		blog.Comments = make([]*blogpb.Comment, 0)
	}

	blog.Comments = append(blog.Comments, rqst.Comment)

	err = svr.saveBlogPost(clientId, blog)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Return the comment itself...
	return &blogpb.AddCommentResponse{Comment: rqst.Comment}, nil
}

// Remove comment from a post or comment.
func (svr *server) RemoveComment(ctx context.Context, rqst *blogpb.RemoveCommentRequest) (*blogpb.RemoveCommentResponse, error) {
	return nil, nil
}
