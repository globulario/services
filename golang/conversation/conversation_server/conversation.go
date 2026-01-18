package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/conversation/conversationpb"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

//////////////// Conversation DB connection helpers ////////////////

/*
getConversationConnection returns (and caches) a Badger store handle
for a given conversation ID. Databases are created under:
<srv.Root>/conversations/<id>
*/
func (srv *server) getConversationConnection(id string) (*storage_store.Badger_store, error) {
	dbPath := srv.Root + "/conversations/" + id
	Utility.CreateDirIfNotExist(dbPath)

	// Use sync.Map semantics (as in the original code) to avoid races.
	if v, ok := srv.conversations.Load(dbPath); ok == true {
		return v.(*storage_store.Badger_store), nil
	}

	conn := storage_store.NewBadger_store()
	srv.conversations.Store(dbPath, conn)
	slog.Info("conversation db opened", "path", dbPath)
	return conn, nil
}

func (srv *server) closeConversationConnection(id string) {
	dbPath := srv.Root + "/conversations/" + id
	v, found := srv.conversations.Load(dbPath)
	if found == false {
		return
	}

	v.(*storage_store.Badger_store).Close()
	srv.conversations.Delete(dbPath)
	slog.Info("conversation db closed", "path", dbPath)
}

//////////////// Conversations listing helpers /////////////////////

func (srv *server) getConversations(accountId string) (*conversationpb.Conversations, error) {
	b, err := srv.store.GetItem(accountId + "_conversations")
	if err != nil {
		// Treat missing list as empty rather than internal failure.
		slog.Warn("getConversations: no list found, returning empty", "account", accountId, "err", err)
		return &conversationpb.Conversations{Conversations: []*conversationpb.Conversation{}}, nil
	}

	out := &conversationpb.Conversations{Conversations: []*conversationpb.Conversation{}}
	uuids := []string{}
	_ = json.Unmarshal(b, &uuids)

	for _, id := range uuids {
		if c, err := srv.getConversation(id); err == nil {
			out.Conversations = append(out.Conversations, c)
		} else {
			slog.Error("getConversations: failed to load conversation", "id", id, "err", err)
		}
	}
	return out, nil
}

func (srv *server) addConversationParticipant(participant, conversation string) error {
	c, err := srv.getConversation(conversation)
	if err != nil {
		return err
	}
	if !Utility.Contains(c.Participants, participant) {
		c.Participants = append(c.Participants, participant)
		slog.Info("participant added to conversation", "participant", participant, "conversation", conversation)
		return srv.saveConversation(c)
	}
	return nil
}

func (srv *server) removeConversationParticipant(participant, conversation string) error {
	c, err := srv.getConversation(conversation)
	if err != nil {
		return err
	}
	if !Utility.Contains(c.Participants, participant) {
		return nil
	}
	next := []string{}
	for _, p := range c.Participants {
		if p != participant {
			next = append(next, p)
		}
	}
	c.Participants = next
	slog.Info("participant removed from conversation", "participant", participant, "conversation", conversation)
	return srv.saveConversation(c)
}

func (srv *server) addParticipantConversation(participant, conversation string) error {
	// index by participant
	b, err := srv.store.GetItem(participant + "_conversations")
	list := []string{}
	if err == nil {
		if err := json.Unmarshal(b, &list); err != nil {
			return err
		}
	}
	if Utility.Contains(list, conversation) {
		return nil
	}
	list = append(list, conversation)
	j, err := json.Marshal(list)
	if err != nil {
		return err
	}
	slog.Info("conversation indexed for participant", "participant", participant, "conversation", conversation)
	return srv.store.SetItem(participant+"_conversations", j)
}

func (srv *server) removeParticipantConversation(participant, conversation string) error {
	b, err := srv.store.GetItem(participant + "_conversations")
	list := []string{}
	if err == nil {
		if err := json.Unmarshal(b, &list); err != nil {
			return err
		}
	}
	next := []string{}
	for _, v := range list {
		if v != conversation {
			next = append(next, v)
		}
	}
	j, err := json.Marshal(next)
	if err != nil {
		return err
	}
	slog.Info("conversation unindexed for participant", "participant", participant, "conversation", conversation)
	return srv.store.SetItem(participant+"_conversations", j)
}

//////////////// Public RPCs – conversations ///////////////////////

/*
CreateConversation creates a new conversation owned by the authenticated user
and returns the created Conversation.
*/
func (srv *server) CreateConversation(ctx context.Context, rqst *conversationpb.CreateConversationRequest) (*conversationpb.CreateConversationResponse, error) {
	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.RandomUUID()
	if len(rqst.Language) == 0 {
		rqst.Language = "en"
	}

	c := &conversationpb.Conversation{
		Uuid:            uuid,
		Name:            rqst.Name,
		Keywords:        rqst.Keywords,
		CreationTime:    time.Now().Unix(),
		LastMessageTime: 0,
		Language:        rqst.Language,
		Participants:    []string{clientId},
		Mac:             srv.Mac,
	}

	if err := srv.saveConversation(c); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.addParticipantConversation(clientId, uuid); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.addResourceOwner(token, uuid, clientId, "conversation", rbacpb.SubjectType_ACCOUNT); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	slog.Info("conversation created", "uuid", uuid, "owner", clientId, "name", rqst.Name)
	return &conversationpb.CreateConversationResponse{Conversation: c}, nil
}

/*
GetConversations returns all conversations in which the given account is indexed as a participant.
*/
func (srv *server) GetConversations(ctx context.Context, rqst *conversationpb.GetConversationsRequest) (*conversationpb.GetConversationsResponse, error) {
	cs, err := srv.getConversations(rqst.Creator)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &conversationpb.GetConversationsResponse{Conversations: cs}, nil
}

/*
KickoutFromConversation removes an account from a conversation. Only the owner may kick others.
*/
func (srv *server) KickoutFromConversation(ctx context.Context, rqst *conversationpb.KickoutFromConversationRequest) (*conversationpb.KickoutFromConversationResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if _, err := srv.getConversation(rqst.ConversationUuid); err != nil {
		return nil, status.Errorf(codes.NotFound, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	isOwner, _, err := srv.validateAccess(clientId, rbacpb.SubjectType_ACCOUNT, "owner", rqst.ConversationUuid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if !isOwner {
		return nil, status.Errorf(codes.PermissionDenied, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("only the owner of the conversation can kick out a participant")))
	}

	if err := srv.removeConversationParticipant(rqst.Account, rqst.ConversationUuid); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.removeParticipantConversation(rqst.Account, rqst.ConversationUuid); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	slog.Info("participant kicked out", "conversation", rqst.ConversationUuid, "by", clientId, "account", rqst.Account)
	return &conversationpb.KickoutFromConversationResponse{}, nil
}

func (srv *server) deleteConversation(token, clientId string, c *conversationpb.Conversation) error {
	if err := srv.removeConversationParticipant(clientId, c.Uuid); err != nil {
		return err
	}
	if err := srv.removeParticipantConversation(clientId, c.Uuid); err != nil {
		return err
	}

	for _, p := range c.Participants {
		_ = srv.publish(`kickout_conversation_`+c.Uuid+`_evt`, []byte(p))
	}

	srv.closeConversationConnection(c.Uuid)

	if Utility.Exists(srv.Root + "/conversations/" + c.Uuid) {
		if err := os.RemoveAll(srv.Root + "/conversations/" + c.Uuid); err != nil {
			return err
		}
	}

	if err := srv.search_engine.DeleteDocument(srv.Root+"/conversations/search_data", c.Uuid); err != nil {
		return err
	}

	if c.Invitations != nil {
		for _, inv := range c.Invitations.Invitations {
			_ = srv.removeInvitation(inv)
		}
	}

	for _, p := range c.Participants {
		_ = srv.removeParticipantConversation(p, c.Uuid)
	}

	if err := srv.store.RemoveItem(c.Uuid); err != nil {
		return err
	}

	_ = srv.publish(`delete_conversation_`+c.Uuid+`_evt`, []byte(c.Uuid))

	if err := srv.deleteResourcePermissions(token, c.Uuid); err != nil {
		return err
	}

	slog.Info("conversation deleted", "uuid", c.Uuid, "by", clientId)
	return nil
}

/*
DeleteConversation deletes the conversation if the caller is the owner; otherwise
the caller simply leaves the conversation.
*/
func (srv *server) DeleteConversation(ctx context.Context, rqst *conversationpb.DeleteConversationRequest) (*conversationpb.DeleteConversationResponse, error) {
	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if _, _, err := srv.validateAccess(clientId, rbacpb.SubjectType_ACCOUNT, "owner", rqst.ConversationUuid); err != nil {
		// Not owner → leave
		if err := srv.removeConversationParticipant(clientId, rqst.ConversationUuid); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if err := srv.removeParticipantConversation(clientId, rqst.ConversationUuid); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		slog.Info("non-owner left conversation", "conversation", rqst.ConversationUuid, "account", clientId)
		return &conversationpb.DeleteConversationResponse{}, nil
	}

	c, err := srv.getConversation(rqst.ConversationUuid)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err := srv.deleteConversation(token, clientId, c); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &conversationpb.DeleteConversationResponse{}, nil
}

/*
FindConversations searches conversations by name/keywords using the service search_engine.
*/
func (srv *server) FindConversations(ctx context.Context, rqst *conversationpb.FindConversationsRequest) (*conversationpb.FindConversationsResponse, error) {
	paths := []string{srv.Root + "/conversations/search_data"}
	results, err := srv.search_engine.SearchDocuments(paths, rqst.Language, []string{"name", "keywords"}, rqst.Query, rqst.Offset, rqst.PageSize, rqst.SnippetSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	list := []*conversationpb.Conversation{}
	for _, r := range results.Results {
		c := new(conversationpb.Conversation)
		if err := protojson.Unmarshal([]byte(r.Data), c); err == nil {
			list = append(list, c)
		}
	}
	return &conversationpb.FindConversationsResponse{Conversations: list}, nil
}

//////////////// Streams (Connect / Join) //////////////////////////

/*
Connect establishes a control stream for the client (used for notifications and
server push). The caller must pass a valid JWT in gRPC metadata key "token".
*/
func (srv *server) Connect(rqst *conversationpb.ConnectRequest, stream conversationpb.ConversationService_ConnectServer) error {
	var clientId string
	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			if len(claims.UserDomain) == 0 {
				return status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no user domain found in token")))
			}
			clientId = claims.ID + "@" + claims.UserDomain
		} else {
			return status.Errorf(codes.Unauthenticated, "Connect: no token was given")
		}
	}

	action := map[string]interface{}{
		"action":   "connect",
		"stream":   stream,
		"uuid":     rqst.Uuid,
		"clientId": clientId,
		"quit":     make(chan bool),
	}
	srv.actions <- action
	<-action["quit"].(chan bool)

	slog.Info("client connected", "uuid", rqst.Uuid, "clientId", clientId)
	return nil
}

/*
Disconnect ends the control stream for the authenticated client.
*/
func (srv *server) Disconnect(ctx context.Context, rqst *conversationpb.DisconnectRequest) (*conversationpb.DisconnectResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	quit := map[string]interface{}{"action": "disconnect", "uuid": rqst.Uuid, "clientId": clientId}
	srv.actions <- quit
	slog.Info("client disconnected", "uuid", rqst.Uuid, "clientId", clientId)
	return &conversationpb.DisconnectResponse{Result: true}, nil
}

/*
JoinConversation streams the backlog (if any) and then attaches the client to
live messages for the given conversation. A valid JWT must be sent via metadata.
*/
func (srv *server) JoinConversation(rqst *conversationpb.JoinConversationRequest, stream conversationpb.ConversationService_JoinConversationServer) error {
	var clientId string
	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			if len(claims.UserDomain) == 0 {
				return status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no user domain found in token")))
			}
			clientId = claims.ID + "@" + claims.UserDomain
		} else {
			return status.Errorf(codes.Unauthenticated, "JoinConversation: no token was given")
		}
	}

	join := map[string]interface{}{"action": "join", "name": rqst.ConversationUuid, "uuid": rqst.ConnectionUuid, "clientId": clientId}
	srv.actions <- join

	conn, err := srv.getConversationConnection(rqst.ConversationUuid)
	if err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err := srv.addConversationParticipant(clientId, rqst.ConversationUuid); err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	conv, err := srv.getConversation(rqst.ConversationUuid)
	if err != nil {
		return status.Errorf(codes.NotFound, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Backlog (if any)
	if data, err := conn.GetItem(rqst.ConversationUuid + "/*"); err == nil && data != nil {
		results := []interface{}{}
		if err := json.Unmarshal(data, &results); err != nil {
			return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if len(results) == 0 {
			_ = stream.Send(&conversationpb.JoinConversationResponse{Msg: nil, Conversation: conv})
			slog.Info("join conversation (empty backlog)", "conversation", rqst.ConversationUuid, "clientId", clientId)
			return nil
		}
		for i := range results {
			m, err := srv.getMessage(
				results[i].(map[string]interface{})["conversation"].(string),
				results[i].(map[string]interface{})["uuid"].(string),
			)
			if err != nil {
				return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			if i == 0 {
				_ = stream.Send(&conversationpb.JoinConversationResponse{Msg: m, Conversation: conv})
			} else {
				_ = stream.Send(&conversationpb.JoinConversationResponse{Msg: m})
			}
		}
	}

	slog.Info("join conversation (backlog sent)", "conversation", rqst.ConversationUuid, "clientId", clientId)
	return nil
}

/*
LeaveConversation detaches the authenticated user from the given conversation.
*/
func (srv *server) LeaveConversation(ctx context.Context, rqst *conversationpb.LeaveConversationRequest) (*conversationpb.LeaveConversationResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	leave := map[string]interface{}{"action": "leave", "name": rqst.ConversationUuid, "uuid": rqst.ConnectionUuid, "clientId": clientId}
	srv.actions <- leave

	if err := srv.removeConversationParticipant(clientId, rqst.ConversationUuid); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	conv, err := srv.getConversation(rqst.ConversationUuid)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	slog.Info("left conversation", "conversation", rqst.ConversationUuid, "clientId", clientId)
	return &conversationpb.LeaveConversationResponse{Conversation: conv}, nil
}

//////////////// Invitations ///////////////////////////////////////

/*
SendInvitation sends an invitation from the authenticated user to another user to join a conversation.
Caller must be the owner of the conversation.
*/
func (srv *server) SendInvitation(ctx context.Context, rqst *conversationpb.SendInvitationRequest) (*conversationpb.SendInvitationResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if clientId != rqst.Invitation.From {
		return nil, status.Errorf(codes.PermissionDenied, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("invitation must be sent by the authenticated user")))
	}

	domain, _ := config.GetDomain()
	hasAccess, _, err := srv.validateAccess(clientId+"@"+domain, rbacpb.SubjectType_ACCOUNT, "owner", rqst.Invitation.Conversation)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if !hasAccess {
		return nil, status.Errorf(codes.PermissionDenied, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("only the owner can invite users")))
	}

	conv, err := srv.getConversation(rqst.Invitation.Conversation)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if Utility.Contains(conv.Participants, rqst.Invitation.To) {
		return nil, status.Errorf(codes.AlreadyExists, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New(rqst.Invitation.To+" is already participating")))
	}

	if conv.Invitations != nil {
		for _, inv := range conv.Invitations.Invitations {
			if inv.From == rqst.Invitation.From && inv.To == rqst.Invitation.To {
				return nil, status.Errorf(codes.AlreadyExists, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New(rqst.Invitation.To+" is already invited")))
			}
		}
	} else {
		conv.Invitations = &conversationpb.Invitations{Invitations: []*conversationpb.Invitation{}}
	}

	rqst.Invitation.InvitationDate = time.Now().Unix()
	rqst.Invitation.Mac = srv.Mac

	// sent
	sentBytes, err := srv.store.GetItem(clientId + "_sent_invitations")
	sent := &conversationpb.Invitations{Invitations: []*conversationpb.Invitation{}}
	if err == nil {
		_ = protojson.Unmarshal(sentBytes, sent)
	}
	sent.Invitations = append(sent.Invitations, rqst.Invitation)
	if js, err := protojson.Marshal(sent); err == nil {
		_ = srv.store.SetItem(clientId+"_sent_invitations", []byte(js))
	}

	// received
	rcvBytes, err := srv.store.GetItem(rqst.Invitation.To + "_received_invitations")
	rcv := &conversationpb.Invitations{Invitations: []*conversationpb.Invitation{}}
	if err == nil {
		_ = protojson.Unmarshal(rcvBytes, rcv)
	}
	rcv.Invitations = append(rcv.Invitations, rqst.Invitation)
	if js, err := protojson.Marshal(rcv); err == nil {
		_ = srv.store.SetItem(rqst.Invitation.To+"_received_invitations", []byte(js))
	}

	conv.Invitations.Invitations = append(conv.Invitations.Invitations, rqst.Invitation)
	if err := srv.saveConversation(conv); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	slog.Info("invitation sent", "conversation", rqst.Invitation.Conversation, "from", rqst.Invitation.From, "to", rqst.Invitation.To)
	return &conversationpb.SendInvitationResponse{}, nil
}

func (srv *server) removeInvitation(inv *conversationpb.Invitation) error {
	// sent
	sentBytes, err := srv.store.GetItem(inv.From + "_sent_invitations")
	if err != nil {
		return err
	}
	sent := new(conversationpb.Invitations)
	if err := protojson.Unmarshal(sentBytes, sent); err != nil {
		return err
	}
	nextSent := []*conversationpb.Invitation{}
	for _, s := range sent.Invitations {
		if !(s.To == inv.To && s.From == inv.From && s.Conversation == inv.Conversation) {
			nextSent = append(nextSent, s)
		}
	}
	sent.Invitations = nextSent
	if js, err := protojson.Marshal(sent); err == nil {
		if err := srv.store.SetItem(inv.From+"_sent_invitations", js); err != nil {
			return err
		}
	}

	// received
	rcvBytes, err := srv.store.GetItem(inv.To + "_received_invitations")
	if err != nil {
		return err
	}
	rcv := new(conversationpb.Invitations)
	if err := protojson.Unmarshal(rcvBytes, rcv); err != nil {
		return err
	}
	nextRcv := []*conversationpb.Invitation{}
	for _, r := range rcv.Invitations {
		if !(r.To == inv.To && r.From == inv.From && r.Conversation == inv.Conversation) {
			nextRcv = append(nextRcv, r)
		}
	}
	rcv.Invitations = nextRcv
	if js, err := protojson.Marshal(rcv); err == nil {
		if err := srv.store.SetItem(inv.To+"_received_invitations", []byte(js)); err != nil {
			return err
		}
	}

	// conversation copy
	conv, err := srv.getConversation(inv.Conversation)
	if err != nil {
		return err
	}
	next := []*conversationpb.Invitation{}
	for _, v := range conv.Invitations.Invitations {
		if !(v.To == inv.To && v.From == inv.From && v.Conversation == inv.Conversation) {
			next = append(next, v)
		}
	}
	conv.Invitations.Invitations = next
	if err := srv.saveConversation(conv); err != nil {
		return err
	}

	slog.Info("invitation removed", "conversation", inv.Conversation, "from", inv.From, "to", inv.To)
	return nil
}

/*
AcceptInvitation accepts an invitation for the authenticated user.
*/
func (srv *server) AcceptInvitation(ctx context.Context, rqst *conversationpb.AcceptInvitationRequest) (*conversationpb.AcceptInvitationResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if clientId != rqst.Invitation.To {
		return nil, status.Errorf(codes.PermissionDenied, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("you are not authenticated as the invitation recipient")))
	}
	if err := srv.removeInvitation(rqst.Invitation); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.addParticipantConversation(rqst.Invitation.To, rqst.Invitation.Conversation)
	slog.Info("invitation accepted", "conversation", rqst.Invitation.Conversation, "account", rqst.Invitation.To)
	return &conversationpb.AcceptInvitationResponse{}, nil
}

/*
DeclineInvitation declines an invitation for the authenticated user.
*/
func (srv *server) DeclineInvitation(ctx context.Context, rqst *conversationpb.DeclineInvitationRequest) (*conversationpb.DeclineInvitationResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if clientId != rqst.Invitation.To {
		return nil, status.Errorf(codes.PermissionDenied, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("you are not authenticated as the invitation recipient")))
	}
	if err := srv.removeInvitation(rqst.Invitation); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	slog.Info("invitation declined", "conversation", rqst.Invitation.Conversation, "account", rqst.Invitation.To)
	return &conversationpb.DeclineInvitationResponse{}, nil
}

/*
RevokeInvitation revokes an invitation previously sent by the authenticated user.
*/
func (srv *server) RevokeInvitation(ctx context.Context, rqst *conversationpb.RevokeInvitationRequest) (*conversationpb.RevokeInvitationResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if clientId != rqst.Invitation.From {
		return nil, status.Errorf(codes.PermissionDenied, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("you are not authenticated as the invitation sender")))
	}
	if err := srv.removeInvitation(rqst.Invitation); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	slog.Info("invitation revoked", "conversation", rqst.Invitation.Conversation, "by", rqst.Invitation.From)
	return &conversationpb.RevokeInvitationResponse{}, nil
}

/*
GetReceivedInvitations returns invitations received by the authenticated account.
*/
func (srv *server) GetReceivedInvitations(ctx context.Context, rqst *conversationpb.GetReceivedInvitationsRequest) (*conversationpb.GetReceivedInvitationsResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if clientId != rqst.Account {
		return nil, status.Errorf(codes.PermissionDenied, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("you are not authenticated as "+rqst.Account)))
	}
	b, err := srv.store.GetItem(clientId + "_received_invitations")
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	rcv := new(conversationpb.Invitations)
	if err := protojson.Unmarshal(b, rcv); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &conversationpb.GetReceivedInvitationsResponse{Invitations: rcv}, nil
}

/*
GetSentInvitations returns invitations sent by the authenticated account.
*/
func (srv *server) GetSentInvitations(ctx context.Context, rqst *conversationpb.GetSentInvitationsRequest) (*conversationpb.GetSentInvitationsResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if clientId != rqst.Account {
		return nil, status.Errorf(codes.PermissionDenied, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("you are not authenticated as "+rqst.Account)))
	}
	b, err := srv.store.GetItem(clientId + "_sent_invitations")
	sent := &conversationpb.Invitations{Invitations: []*conversationpb.Invitation{}}
	if err == nil {
		if err := protojson.Unmarshal(b, sent); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	return &conversationpb.GetSentInvitationsResponse{Invitations: sent}, nil
}

//////////////// Messages //////////////////////////////////////////

func (srv *server) sendMessage(msg *conversationpb.Message) error {
	conn, err := srv.getConversationConnection(msg.Conversation)
	if err != nil {
		return err
	}

	js, err := protojson.Marshal(msg)
	if err != nil {
		return err
	}

	if err := conn.SetItem(msg.Conversation+"/"+msg.Uuid, []byte(js)); err != nil {
		return err
	}

	Utility.CreateDirIfNotExist(srv.Root + "/conversations/" + msg.Conversation + "/search_data")
	srv.search_engine.IndexJsonObject(
		srv.Root+"/conversations/"+msg.Conversation+"/search_data",
		string(js), msg.Language, "uuid", []string{"text"}, string(js),
	)

	conv, err := srv.getConversation(msg.Conversation)
	if err != nil {
		return err
	}
	conv.LastMessageTime = time.Now().Unix()
	if err := srv.saveConversation(conv); err != nil {
		return err
	}

	send := map[string]interface{}{"action": "send_message", "name": msg.Conversation, "message": msg}
	srv.actions <- send

	slog.Info("message sent", "conversation", msg.Conversation, "uuid", msg.Uuid, "author", msg.Author)
	return nil
}

/*
SendMessage persists and broadcasts a message to participants of a conversation.
*/
func (srv *server) SendMessage(ctx context.Context, rqst *conversationpb.SendMessageRequest) (*conversationpb.SendMessageResponse, error) {
	if err := srv.sendMessage(rqst.Msg); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &conversationpb.SendMessageResponse{}, nil
}

/*
DeleteMessage removes a specific message from a conversation.
*/
func (srv *server) DeleteMessage(ctx context.Context, rqst *conversationpb.DeleteMessageRequest) (*conversationpb.DeleteMessageResponse, error) {
	if err := srv.deleteMessages(rqst.Conversation, rqst.Uuid); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	slog.Info("message deleted", "conversation", rqst.Conversation, "uuid", rqst.Uuid)
	return &conversationpb.DeleteMessageResponse{}, nil
}

/*
FindMessages streams messages that match a query. (Not implemented in original code.)
*/
func (srv *server) FindMessages(rqst *conversationpb.FindMessagesRequest, stream conversationpb.ConversationService_FindMessagesServer) error {
	return status.Errorf(codes.Unimplemented, "FindMessages is not implemented")
}

func (srv *server) getMessage(conversation, uuid string) (*conversationpb.Message, error) {
	conn, err := srv.getConversationConnection(conversation)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	data, err := conn.GetItem(conversation + "/" + uuid)
	if err != nil {
		return nil, err
	}
	msg := new(conversationpb.Message)
	if err := protojson.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (srv *server) deleteMessages(conversation, uuid string) error {
	conn, err := srv.getConversationConnection(conversation)
	if err != nil {
		return err
	}
	return conn.RemoveItem(conversation + "/" + uuid + "*")
}

/*
LikeMessage toggles a 'like' on a message by the authenticated account.
*/
func (srv *server) LikeMessage(ctx context.Context, rqst *conversationpb.LikeMessageRqst) (*conversationpb.LikeMessageResponse, error) {
	msg, err := srv.getMessage(rqst.Conversation, rqst.Message)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if msg.Author == rqst.Account {
		return nil, status.Errorf(codes.FailedPrecondition, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("cannot like your own message")))
	}
	if !Utility.Contains(msg.Likes, rqst.Account) {
		msg.Dislikes = Utility.RemoveString(msg.Dislikes, rqst.Account)
		msg.Likes = append(msg.Likes, rqst.Account)
	} else {
		msg.Likes = Utility.RemoveString(msg.Likes, rqst.Account)
	}
	if err := srv.sendMessage(msg); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &conversationpb.LikeMessageResponse{}, nil
}

/*
DislikeMessage toggles a 'dislike' on a message by the authenticated account.
*/
func (srv *server) DislikeMessage(ctx context.Context, rqst *conversationpb.DislikeMessageRqst) (*conversationpb.DislikeMessageResponse, error) {
	msg, err := srv.getMessage(rqst.Conversation, rqst.Message)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if msg.Author == rqst.Account {
		return nil, status.Errorf(codes.FailedPrecondition, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("cannot dislike your own message")))
	}
	if !Utility.Contains(msg.Dislikes, rqst.Account) {
		msg.Likes = Utility.RemoveString(msg.Likes, rqst.Account)
		msg.Dislikes = append(msg.Dislikes, rqst.Account)
	} else {
		msg.Dislikes = Utility.RemoveString(msg.Dislikes, rqst.Account)
	}
	if err := srv.sendMessage(msg); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &conversationpb.DislikeMessageResponse{}, nil
}

/*
SetMessageRead marks messages as read for the authenticated account.
(Not implemented in original code.)
*/
func (srv *server) SetMessageRead(ctx context.Context, rqst *conversationpb.SetMessageReadRqst) (*conversationpb.SetMessageReadResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "SetMessageRead is not implemented")
}

//////////////// Single conversation helpers //////////////////////

func (srv *server) getConversation(uuid string) (*conversationpb.Conversation, error) {
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, err
	}
	c := new(conversationpb.Conversation)
	if err := protojson.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

/*
GetConversation returns the conversation by its ID.
*/
func (srv *server) GetConversation(ctx context.Context, rqst *conversationpb.GetConversationRequest) (*conversationpb.GetConversationResponse, error) {
	c, err := srv.getConversation(rqst.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &conversationpb.GetConversationResponse{Conversation: c}, nil
}

func (srv *server) saveConversation(c *conversationpb.Conversation) error {
	js, err := protojson.Marshal(c)
	if err != nil {
		return err
	}
	if err := srv.store.SetItem(c.Uuid, []byte(js)); err != nil {
		return err
	}
	slog.Info("conversation saved", "uuid", c.Uuid, "name", c.Name)
	return srv.search_engine.IndexJsonObject(
		srv.Root+"/conversations/search_data",
		string(js), c.Language, "uuid", []string{"name", "keywords"}, string(js),
	)
}
