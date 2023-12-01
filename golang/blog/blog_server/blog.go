package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	//"fmt"
	"sort"
	"time"

	"github.com/blevesearch/bleve"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/blog/blogpb"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/////////////////////// Blog specific function /////////////////////////////////

// One request followed by one response
func (srv *server) CreateBlogPost(ctx context.Context, rqst *blogpb.CreateBlogPostRequest) (*blogpb.CreateBlogPostResponse, error) {

	// So here I will create a new blog from the infromation sent by the user.
	// Get validated user id and token.
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
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

	// Save the blog.
	err = srv.saveBlogPost(clientId, blogPost)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Set the owner of the conversation.
	err = srv.addResourceOwner( uuid, "blog", clientId, rbacpb.SubjectType_ACCOUNT)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will index the blog post...
	// So here Will create the indexation for the movie...
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Index the title and put it in the search engine.
	err = index.Index(uuid, blogPost)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Associated original object here...
	text := blogPost.Text
	blogPost.Text = "" // empty the text to save in the search engine...

	jsonStr, err := protojson.Marshal(blogPost)

	if err == nil {
		err = index.SetInternal([]byte(uuid), []byte(jsonStr))
	}

	// set back to the response.
	blogPost.Text = text

	// TODO send publish event also..
	return &blogpb.CreateBlogPostResponse{BlogPost: blogPost}, nil
}

// Update a blog post...
func (srv *server) SaveBlogPost(ctx context.Context, rqst *blogpb.SaveBlogPostRequest) (*blogpb.SaveBlogPostResponse, error) {
	
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Save the blog.
	err = srv.saveBlogPost(clientId, rqst.BlogPost)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will index the blog post...
	// So here Will create the indexation for the movie...
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Index the title and put it in the search engine.
	err = index.Index(rqst.BlogPost.Uuid, rqst.BlogPost)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Associated original object here...
	rqst.BlogPost.Text = "" // emoty the text for the internal object...

	jsonStr, err := protojson.Marshal(rqst.BlogPost)

	if err == nil {
		err = index.SetInternal([]byte(rqst.BlogPost.Uuid), []byte(jsonStr))
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// TODO send publish event also..
	return &blogpb.SaveBlogPostResponse{}, nil
}

// Retreive Blog Post by author
func (srv *server) GetBlogPostsByAuthors(rqst *blogpb.GetBlogPostsByAuthorsRequest, stream blogpb.BlogService_GetBlogPostsByAuthorsServer) error {

	// Retreive the list of all blogs.
	blogs := make([]*blogpb.BlogPost, 0)
	for i := 0; i < len(rqst.Authors); i++ {
		blogs_, err := srv.getBlogPostByAuthor(rqst.Authors[i])
		if err == nil {
			blogs = append(blogs, blogs_...)
		}
	}

	if len(blogs) == 0 {
		return errors.New("no blog founds")
	} else if len(blogs) > 1 {
		// Now I will sort the values by creation date...
		sort.Slice(blogs, func(i, j int) bool {
			return blogs[i].CreationTime < blogs[j].CreationTime
		})

	}

	// Finaly I will return blogs..
	max := int(rqst.Max)

	// In that case I will return the whole list.
	for i := 0; i < max && i < len(blogs); i++ {
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

// Search blog by id's or text find in the post...
func (srv *server) GetBlogPosts(rqst *blogpb.GetBlogPostsRequest, stream blogpb.BlogService_GetBlogPostsServer) error {
	// So here I will return the list of blogs that match the uuid's...
	for i := 0; i < len(rqst.Uuids); i++ {
		data, err := srv.store.GetItem(rqst.Uuids[i])
		if err == nil {
			b := new(blogpb.BlogPost)
			err := protojson.Unmarshal(data, b)

			if err != nil {
				fmt.Println("fail to unmarchal blog with uuid:", rqst.Uuids[i])
			} else {
				// Here I will send the blog post...
				stream.Send(&blogpb.GetBlogPostsResponse{BlogPost: b})
			}

		} else {
			fmt.Println("fail to retreive blog with uuid:", rqst.Uuids[i])
		}
	}

	return nil
}

// Search blog by keyword's or text find in the post...
func (srv *server) SearchBlogPosts(rqst *blogpb.SearchBlogPostsRequest, stream blogpb.BlogService_SearchBlogPostsServer) error {

	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	query := bleve.NewQueryStringQuery(rqst.Query)
	request := bleve.NewSearchRequest(query)
	request.Size = int(rqst.Size) //
	request.From = int(rqst.Offset)
	if request.Size == 0 {
		request.Size = 50
	}

	// Now I will add the facets for type and genre.

	// The genre facet.
	tags := bleve.NewFacetRequest("Keywords", int(rqst.Size))
	request.AddFacet("Keywords", tags)

	request.Highlight = bleve.NewHighlightWithStyle("html")
	request.Fields = rqst.Fields
	result, err := index.Search(request)

	if err != nil { // an empty query would cause this
		return err
	}

	// The first return message will be the summary of the result...
	summary := new(blogpb.SearchSummary)
	summary.Query = rqst.Query // set back the input query.
	summary.Took = result.Took.Milliseconds()
	summary.Total = result.Total

	// Here I will send the summary...
	stream.Send(&blogpb.SearchBlogPostsResponse{
		Result: &blogpb.SearchBlogPostsResponse_Summary{
			Summary: summary,
		},
	})

	// Now I will generate the hits informations...
	for i, hit := range result.Hits {
		id := hit.ID
		hit_ := new(blogpb.SearchHit)
		hit_.Score = hit.Score
		hit_.Index = int32(i)
		hit_.Snippets = make([]*blogpb.Snippet, 0)

		// Now I will extract fragment for fields...
		for fragmentField, fragments := range hit.Fragments {
			snippet := new(blogpb.Snippet)
			snippet.Field = fragmentField
			snippet.Fragments = make([]string, 0)
			for _, fragment := range fragments {
				snippet.Fragments = append(snippet.Fragments, fragment)
			}
			// append to the results.
			hit_.Snippets = append(hit_.Snippets, snippet)
		}

		// Here I will get the title itself.
		raw, err := index.GetInternal([]byte(id))
		if err == nil {
			blogPost := new(blogpb.BlogPost)
			err = protojson.Unmarshal(raw, blogPost)
			if err == nil {
				hit_.Blog = blogPost
				// Here I will send the search result...
				stream.Send(&blogpb.SearchBlogPostsResponse{
					Result: &blogpb.SearchBlogPostsResponse_Hit{
						Hit: hit_,
					},
				})
			}
		}
	}

	// Finaly I will send the facets...
	facets := new(blogpb.SearchFacets)
	facets.Facets = make([]*blogpb.SearchFacet, 0)
	for _, f := range result.Facets {
		facet_ := new(blogpb.SearchFacet)
		facet_.Field = f.Field
		facet_.Total = int32(f.Total)
		facet_.Other = int32(f.Other)
		facet_.Terms = make([]*blogpb.SearchFacetTerm, 0)
		// Regular terms...
		for _, t := range f.Terms {
			term := new(blogpb.SearchFacetTerm)
			term.Count = int32(t.Count)
			term.Term = t.Term
			facet_.Terms = append(facet_.Terms, term)
		}
		facets.Facets = append(facets.Facets, facet_)
	}

	// send the facets...
	stream.Send(&blogpb.SearchBlogPostsResponse{
		Result: &blogpb.SearchBlogPostsResponse_Facets{
			Facets: facets,
		},
	})

	return nil
}

// Delete a blog.
func (srv *server) DeleteBlogPost(ctx context.Context, rqst *blogpb.DeleteBlogPostRequest) (*blogpb.DeleteBlogPostResponse, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	err = srv.deleteBlogPost(clientId, rqst.Uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will remove it from the search engine.
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.Delete(rqst.Uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.DeleteInternal([]byte(rqst.Uuid))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &blogpb.DeleteBlogPostResponse{}, nil
}

// Emoji a post or comment
func (srv *server) AddEmoji(ctx context.Context, rqst *blogpb.AddEmojiRequest) (*blogpb.AddEmojiResponse, error) {
	
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
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
	blog, err := srv.getBlogPost(rqst.Uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if blog.Uuid == rqst.Emoji.Parent {
		// Here I will append the emoji to the blog itself...
		if blog.Emotions == nil {
			blog.Emotions = make([]*blogpb.Emoji, 0)
		}

		blog.Emotions = append(blog.Emotions, rqst.Emoji)
	} else {
		// Here I will find the comment who must contain the emoji
		comment, err := srv.getBlogComment(rqst.Emoji.Parent, blog)
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

	err = srv.saveBlogPost(blog.Author, blog)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &blogpb.AddEmojiResponse{Emoji: rqst.Emoji}, nil
}

// Remove like from a post or comment.
func (srv *server) RemoveEmoji(ctx context.Context, rqst *blogpb.RemoveEmojiRequest) (*blogpb.RemoveEmojiResponse, error) {
	return nil, nil
}

// Comment a post or comment
func (srv *server) AddComment(ctx context.Context, rqst *blogpb.AddCommentRequest) (*blogpb.AddCommentResponse, error) {
	
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	if rqst.Comment.AccountId != clientId {
		err := errors.New("you can't comment for another account")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	blog, err := srv.getBlogPost(rqst.Uuid)
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
	if parentUuid != rqst.Uuid {
		if len(parentUuid) > 0 {

			// Here I need to find the comment in the blog comments...
			if blog.Comments == nil {
				err := errors.New("no parent comment comment found")
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			parentComment, err := srv.getBlogComment(parentUuid, blog)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			// So here the parent comment was found...
			parentComment.Comments = append(parentComment.Comments, rqst.Comment)

		}
	} else {

		// Directly comment the blog...
		if blog.Comments == nil {
			blog.Comments = make([]*blogpb.Comment, 0)
		}

		blog.Comments = append(blog.Comments, rqst.Comment)
	}

	err = srv.saveBlogPost(clientId, blog)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Return the comment itself...
	return &blogpb.AddCommentResponse{Comment: rqst.Comment}, nil
}

// Remove comment from a post or comment.
func (srv *server) RemoveComment(ctx context.Context, rqst *blogpb.RemoveCommentRequest) (*blogpb.RemoveCommentResponse, error) {
	return nil, nil
}
