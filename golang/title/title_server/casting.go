package main

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"

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
	logger.Info("publisher created", "PublisherID", rqst.Publisher.ID)
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
	logger.Info("publisher deleted", "PublisherID", rqst.PublisherID)
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
	clientId, token, err := security.GetClientId(ctx)
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
			_ = srv.createVideo(token, rqst.IndexPath, clientId, video)
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
		if person == nil || person.ID == "" {
			out = append(out, person)
			continue
		}

		existing, err := srv.getPersonById(indexPath, person.ID)
		if err == nil && existing != nil {
			mergeExistingRoleList(person, existing, role)
		}

		roleChanged := appendTitleRole(person, role, titleId)
		needSave := existing == nil || roleChanged
		if existing != nil {
			needSave = roleChanged || mergePersonMetadata(existing, person)
		}

		if needSave {
			slog.Info("update person", "personID", person.ID, "titleID", titleId, "role", role)
			_ = srv.createPerson(indexPath, person)
		}

		out = append(out, person)
	}
	return out
}

func mergePersonMetadata(existing, candidate *titlepb.Person) bool {
	if existing == nil || candidate == nil {
		return true
	}
	changed := false

	if mergeStringField(&candidate.URL, existing.URL) {
		changed = true
	}
	if mergeStringField(&candidate.FullName, existing.FullName) {
		changed = true
	}
	if mergeStringField(&candidate.Picture, existing.Picture) {
		changed = true
	}
	if mergeStringField(&candidate.Biography, existing.Biography) {
		changed = true
	}
	if mergeStringField(&candidate.CareerStatus, existing.CareerStatus) {
		changed = true
	}
	if mergeStringField(&candidate.Gender, existing.Gender) {
		changed = true
	}
	if mergeStringField(&candidate.BirthPlace, existing.BirthPlace) {
		changed = true
	}
	if mergeStringField(&candidate.BirthDate, existing.BirthDate) {
		changed = true
	}

	if aliases, aliasChanged := mergeUniqueStrings(candidate.Aliases, existing.Aliases); aliases != nil {
		candidate.Aliases = aliases
		if aliasChanged {
			changed = true
		}
	}

	return changed
}

func mergeStringField(candidate *string, existing string) bool {
	if candidate == nil {
		return false
	}
	if *candidate == "" {
		*candidate = existing
		return false
	}
	if existing != *candidate {
		return true
	}
	return false
}

func appendTitleRole(person *titlepb.Person, role, titleID string) bool {
	if person == nil || titleID == "" {
		return false
	}
	switch role {
	case "Casting":
		return appendUnique(&person.Casting, titleID)
	case "Acting":
		return appendUnique(&person.Acting, titleID)
	case "Directing":
		return appendUnique(&person.Directing, titleID)
	case "Writing":
		return appendUnique(&person.Writing, titleID)
	default:
		return false
	}
}

func appendUnique(slice *[]string, value string) bool {
	if value == "" || slice == nil {
		return false
	}
	if Utility.Contains(*slice, value) {
		return false
	}
	*slice = append(*slice, value)
	return true
}

func mergeExistingRoleList(person, existing *titlepb.Person, role string) {
	if person == nil || existing == nil {
		return
	}
	switch role {
	case "Casting":
		appendExistingEntries(&person.Casting, existing.Casting)
	case "Acting":
		appendExistingEntries(&person.Acting, existing.Acting)
	case "Directing":
		appendExistingEntries(&person.Directing, existing.Directing)
	case "Writing":
		appendExistingEntries(&person.Writing, existing.Writing)
	}
}

func appendExistingEntries(target *[]string, values []string) {
	if target == nil || len(values) == 0 {
		return
	}
	for _, v := range values {
		appendUnique(target, v)
	}
}

func mergeUniqueStrings(primary, fallback []string) ([]string, bool) {
	if len(primary) == 0 && len(fallback) == 0 {
		return nil, false
	}
	seen := make(map[string]struct{}, len(primary)+len(fallback))
	result := make([]string, 0, len(primary)+len(fallback))
	for _, v := range fallback {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	changed := false
	for _, v := range primary {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
		changed = true
	}
	if len(result) == 0 {
		return nil, false
	}
	return result, changed
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
