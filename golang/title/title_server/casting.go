package main

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"

	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// CreatePublisher inserts a Publisher into the index and its raw JSON in the internal store.
func (srv *server) CreatePublisher(ctx context.Context, rqst *titlepb.CreatePublisherRequest) (*titlepb.CreatePublisherResponse, error) {
	if err := checkNotNil("publisher", rqst.Publisher); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open index: %v", err)
	}
	uuid := Utility.GenerateUUID(rqst.Publisher.ID)
	if err := index.Index(uuid, rqst.Publisher); err != nil {
		return nil, status.Errorf(codes.Internal, "index publisher: %v", err)
	}
	if raw, err := protojson.Marshal(rqst.Publisher); err == nil {
		if err := index.SetInternal([]byte(uuid), raw); err != nil {
			return nil, status.Errorf(codes.Internal, "store raw publisher: %v", err)
		}
	}
	logger.Info("publisher created", "publisherID", rqst.Publisher.ID)
	return &titlepb.CreatePublisherResponse{}, nil
}

// DeletePublisher removes a Publisher from both index and internal store.
func (srv *server) DeletePublisher(ctx context.Context, rqst *titlepb.DeletePublisherRequest) (*titlepb.DeletePublisherResponse, error) {
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open index: %v", err)
	}
	uuid := Utility.GenerateUUID(rqst.PublisherID)
	if err := index.Delete(uuid); err != nil {
		return nil, status.Errorf(codes.Internal, "delete publisher index: %v", err)
	}
	if err := index.DeleteInternal([]byte(uuid)); err != nil {
		return nil, status.Errorf(codes.Internal, "delete publisher raw: %v", err)
	}
	logger.Info("publisher deleted", "publisherID", rqst.PublisherID)
	return &titlepb.DeletePublisherResponse{}, nil
}

// getPublisherById retrieves a Publisher from the internal store using its id.
func (srv *server) getPublisherById(indexPath, id string) (*titlepb.Publisher, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}
	index, err := srv.getIndex(indexPath)
	if err != nil {
		return nil, err
	}
	q := bleve.NewQueryStringQuery(id)
	req := bleve.NewSearchRequest(q)
	res, err := index.Search(req)
	if err != nil {
		return nil, err
	}
	if res.Total == 0 {
		return nil, errors.New("no publisher found with id " + id)
	}
	for _, h := range res.Hits {
		uuid := Utility.GenerateUUID(h.ID)
		raw, err := index.GetInternal([]byte(uuid))
		if err != nil {
			return nil, err
		}
		p := new(titlepb.Publisher)
		if err := protojson.Unmarshal(raw, p); err != nil {
			return nil, err
		}
		return p, nil
	}
	return nil, errors.New("no publisher found with id " + id)
}

// GetPublisherById returns a Publisher by ID.
func (srv *server) GetPublisherById(ctx context.Context, rqst *titlepb.GetPublisherByIdRequest) (*titlepb.GetPublisherByIdResponse, error) {
	p, err := srv.getPublisherById(rqst.IndexPath, rqst.PublisherID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &titlepb.GetPublisherByIdResponse{Publisher: p}, nil
}

// createPerson indexes a Person and stores its raw JSON in the internal store.
func (srv *server) createPerson(indexPath string, person *titlepb.Person) error {
	index, err := srv.getIndex(indexPath)
	if err != nil {
		return err
	}
	if len(person.ID) == 0 || len(person.FullName) == 0 {
		return errors.New("missing information for person")
	}
	uuid := Utility.GenerateUUID(person.ID)
	if !Utility.IsStdBase64(person.Biography) {
		person.Biography = base64.StdEncoding.EncodeToString([]byte(person.Biography))
	}
	if err := index.Index(uuid, person); err != nil {
		return err
	}
	raw, err := protojson.Marshal(person)
	if err != nil {
		return err
	}
	if err := index.SetInternal([]byte(uuid), raw); err != nil {
		return err
	}
	return nil
}

// CreatePerson inserts/updates a Person.
func (srv *server) CreatePerson(ctx context.Context, rqst *titlepb.CreatePersonRequest) (*titlepb.CreatePersonResponse, error) {
	if err := srv.createPerson(rqst.IndexPath, rqst.Person); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	logger.Info("person created", "personID", rqst.Person.GetID())
	return &titlepb.CreatePersonResponse{}, nil
}

// DeletePerson removes a Person and refreshes affected videos' casting lists.
func (srv *server) DeletePerson(ctx context.Context, rqst *titlepb.DeletePersonRequest) (*titlepb.DeletePersonResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open index: %v", err)
	}

	person, err := srv.getPersonById(rqst.IndexPath, rqst.PersonId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	uuid := Utility.GenerateUUID(rqst.PersonId)
	if err := index.Delete(uuid); err != nil {
		return nil, status.Errorf(codes.Internal, "delete person index: %v", err)
	}
	if err := index.DeleteInternal([]byte(uuid)); err != nil {
		return nil, status.Errorf(codes.Internal, "delete person raw: %v", err)
	}
	for _, vid := range person.Casting {
		if video, err := srv.getVideoById(rqst.IndexPath, vid); err == nil {
			_ = srv.createVideo(rqst.IndexPath, clientId, video)
		}
	}
	logger.Info("person deleted", "personID", rqst.PersonId)
	return &titlepb.DeletePersonResponse{}, nil
}

// getPersonById returns a Person by ID from the internal store.
func (srv *server) getPersonById(indexPath, id string) (*titlepb.Person, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}
	index, err := srv.getIndex(indexPath)
	if err != nil {
		return nil, err
	}
	uuid := Utility.GenerateUUID(id)
	raw, err := index.GetInternal([]byte(uuid))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.New("no person found with id " + id)
	}
	p := new(titlepb.Person)
	if err := protojson.Unmarshal(raw, p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetPersonById returns a Person by ID.
func (srv *server) GetPersonById(ctx context.Context, rqst *titlepb.GetPersonByIdRequest) (*titlepb.GetPersonByIdResponse, error) {
	p, err := srv.getPersonById(rqst.IndexPath, rqst.PersonId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &titlepb.GetPersonByIdResponse{Person: p}, nil
}

// saveTitleCasting merges and persists casting info for a title.
func (srv *server) saveTitleCasting(indexPath, titleId, role string, persons []*titlepb.Person) []*titlepb.Person {
	out := make([]*titlepb.Person, 0, len(persons))
	for _, person := range persons {
		if existing, err := srv.getPersonById(indexPath, person.ID); err == nil {
			switch role {
			case "Casting":
				for _, v := range existing.Casting {
					if !Utility.Contains(person.Casting, v) {
						person.Casting = append(person.Casting, v)
					}
				}
				if !Utility.Contains(person.Casting, titleId) {
					person.Casting = append(person.Casting, titleId)
				}
			case "Acting":
				for _, v := range existing.Acting {
					if !Utility.Contains(person.Acting, v) {
						person.Acting = append(person.Acting, v)
					}
				}
				if !Utility.Contains(person.Acting, titleId) {
					person.Acting = append(person.Acting, titleId)
				}
			case "Directing":
				for _, v := range existing.Directing {
					if !Utility.Contains(person.Directing, v) {
						person.Directing = append(person.Directing, v)
					}
				}
				if !Utility.Contains(person.Directing, titleId) {
					person.Directing = append(person.Directing, titleId)
				}
			case "Writing":
				for _, v := range existing.Writing {
					if !Utility.Contains(person.Writing, v) {
						person.Writing = append(person.Writing, v)
					}
				}
				if !Utility.Contains(person.Writing, titleId) {
					person.Writing = append(person.Writing, titleId)
				}
			}
			slog.Info("update person", "personID", person.ID, "titleID", titleId, "role", role)
		}
		_ = srv.createPerson(indexPath, person)
		out = append(out, person)
	}
	return out
}

// SearchPersons queries the people index and streams a summary followed by hits.
func (srv *server) SearchPersons(
	rqst *titlepb.SearchPersonsRequest,
	stream titlepb.TitleService_SearchPersonsServer,
) error {
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		slog.Error("open index failed", "indexPath", rqst.IndexPath, "err", err)
		return status.Errorf(codes.Internal, "open index: %v", err)
	}

	q := bleve.NewQueryStringQuery(rqst.Query)
	req := bleve.NewSearchRequest(q)
	req.Size = int(rqst.Size)
	req.From = int(rqst.Offset)
	if req.Size == 0 {
		req.Size = 50
	}

	// Facets (support both spellings for historical data)
	req.AddFacet("Acting", bleve.NewFacetRequest("Acting", req.Size))
	req.AddFacet("Directing", bleve.NewFacetRequest("Directing", req.Size))
	req.AddFacet("Writting", bleve.NewFacetRequest("Writting", req.Size))
	req.AddFacet("Casting", bleve.NewFacetRequest("Casting", req.Size))

	req.Highlight = bleve.NewHighlightWithStyle("html")
	req.Fields = rqst.Fields

	res, err := index.Search(req)
	if err != nil {
		slog.Error("search persons failed", "query", rqst.Query, "err", err)
		return status.Errorf(codes.Internal, "search persons: %v", err)
	}

	// 1) Send summary
	if err := stream.Send(&titlepb.SearchPersonsResponse{
		Result: &titlepb.SearchPersonsResponse_Summary{
			Summary: &titlepb.SearchSummary{
				Query: rqst.Query,
				Took:  res.Took.Milliseconds(),
				Total: res.Total,
			},
		},
	}); err != nil {
		return status.Errorf(codes.Internal, "send summary: %v", err)
	}

	// 2) Stream hits
	for i, hit := range res.Hits {
		h := &titlepb.SearchHit{
			Score:    hit.Score,
			Index:    int32(i),
			Snippets: make([]*titlepb.Snippet, 0, len(hit.Fragments)),
		}
		for field, frags := range hit.Fragments {
			h.Snippets = append(h.Snippets, &titlepb.Snippet{Field: field, Fragments: frags})
		}

		// Load full person record from internal store and set the oneof.
		if raw, gerr := index.GetInternal([]byte(hit.ID)); gerr == nil && len(raw) > 0 {
			p := new(titlepb.Person)
			if uerr := protojson.Unmarshal(raw, p); uerr == nil {
				h.Result = &titlepb.SearchHit_Person{Person: p}
			} else {
				slog.Warn("unmarshal person failed", "docID", hit.ID, "err", uerr)
			}
		}

		if err := stream.Send(&titlepb.SearchPersonsResponse{
			Result: &titlepb.SearchPersonsResponse_Hit{Hit: h},
		}); err != nil {
			return status.Errorf(codes.Internal, "send hit: %v", err)
		}
	}

	return nil
}
