package imap

import (
	"errors"
	"io"
	"time"

	b64 "encoding/base64"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend/backendutil"
	Utility "github.com/globulario/utility"
)

// -----------------------------------------------------------------------------
// Mailbox implementation
// -----------------------------------------------------------------------------

type MailBox_impl struct {
	name       string
	user       string
	subscribed bool
}

// NewMailBox creates the mailbox metadata for a user if missing and returns a
// MailBox_impl handle. It preserves the original prototype.
func NewMailBox(user string, name string) *MailBox_impl {
	box := new(MailBox_impl)
	box.name = name
	box.user = user

	info := new(imap.MailboxInfo)
	info.Name = name
	info.Delimiter = "/"

	jsonStr, err := Utility.ToJson(info)
	if err != nil {
		logger.Error("NewMailBox: marshal MailboxInfo failed", "user", user, "name", name, "err", err)
		return nil
	}

	connectionId := user + "_db"
	query := `{"Name":"` + name + `"}`
	count, err := Store.Count(connectionId, connectionId, "MailBoxes", query, "")
	if err != nil || count < 1 {
		if _, insErr := Store.InsertOne(connectionId, connectionId, "MailBoxes", jsonStr, ""); insErr != nil {
			logger.Error("NewMailBox: insert mailbox metadata failed",
				"user", user, "name", name, "err", insErr)
		}
	}
	return box
}

// getMailBox returns the mailbox if it exists; otherwise an error.
func getMailBox(user string, name string) (*MailBox_impl, error) {
	box := new(MailBox_impl)
	box.name = name
	box.user = user

	connectionId := user + "_db"
	query := `{"Name":"` + name + `"}`
	count, err := Store.Count(connectionId, connectionId, "MailBoxes", query, "")
	if err != nil || count < 1 {
		if err == nil {
			err = errors.New("mailbox not found")
		}
		logger.Warn("getMailBox: mailbox does not exist", "user", user, "name", name, "err", err)
		return nil, errors.New("No mail box found with name " + name)
	}
	return box, nil
}

// Name returns this mailbox name.
func (mbox *MailBox_impl) Name() string {
	return mbox.name
}

// Info returns this mailbox info loaded from the backend.
func (mbox *MailBox_impl) Info() (*imap.MailboxInfo, error) {
	connectionId := mbox.user + "_db"
	query := `{"Name":"` + mbox.name + `"}`
	infoMap, err := Store.FindOne(connectionId, connectionId, "MailBoxes", query, "")
	if err != nil {
		logger.Error("Info: FindOne failed", "user", mbox.user, "mailbox", mbox.name, "err", err)
		return nil, err
	}

	info := new(imap.MailboxInfo)
	if v, ok := infoMap["Name"].(string); ok {
		info.Name = v
	} else {
		info.Name = mbox.name
	}
	if v, ok := infoMap["Delimiter"].(string); ok {
		info.Delimiter = v
	} else {
		info.Delimiter = "/"
	}
	return info, nil
}

func (mbox *MailBox_impl) uidNext() uint32 {
	var uid uint32
	messages := mbox.getMessages()
	for _, msg := range messages {
		if msg.Uid > uid {
			uid = msg.Uid
		}
	}
	uid++
	return uid
}

// getMessages returns all messages for the mailbox from the backend.
func (mbox *MailBox_impl) getMessages() []*Message {
	messages := make([]*Message, 0)
	connectionId := mbox.user + "_db"

	data, err := Store.Find(connectionId, connectionId, mbox.Name(), "", "")
	if err != nil {
		logger.Error("getMessages: backend Find failed", "user", mbox.user, "mailbox", mbox.name, "err", err)
		return messages
	}

	for i := 0; i < len(data); i++ {
		row, ok := data[i].(map[string]interface{})
		if !ok {
			continue
		}

		m := new(Message)

		// Uid
		if u, ok := row["Uid"].(float64); ok {
			m.Uid = uint32(u)
		}

		// Body (stored as base64-encoded string)
		if s, ok := row["Body"].(string); ok && s != "" {
			if dec, err := b64.StdEncoding.DecodeString(s); err == nil {
				m.Body = dec
			} else {
				logger.Warn("getMessages: base64 decode failed", "uid", m.Uid, "err", err)
			}
		}

		// Flags
		if fl, ok := row["Flags"].([]interface{}); ok {
			m.Flags = make([]string, 0, len(fl))
			for _, f := range fl {
				if fs, ok := f.(string); ok && fs != "" {
					m.Flags = append(m.Flags, fs)
				}
			}
		}

		// Size
		if sz, ok := row["Size"].(float64); ok {
			m.Size = uint32(sz)
		}

		// Date
		if ds, ok := row["Date"].(string); ok && ds != "" {
			// Persisted layout used by prior code.
			const layout = "2020-11-02T01:45:47.764336457Z"
			if t, err := time.Parse(layout, ds); err == nil {
				m.Date = t
			} else {
				// Best effort: try RFC3339
				if t2, err2 := time.Parse(time.RFC3339Nano, ds); err2 == nil {
					m.Date = t2
				} else {
					logger.Warn("getMessages: date parse failed", "uid", m.Uid, "value", ds, "err_primary", err, "err_fallback", err2)
				}
			}
		}

		messages = append(messages, m)
	}

	return messages
}

func (mbox *MailBox_impl) unseenSeqNum() uint32 {
	messages := mbox.getMessages()
	for i, msg := range messages {
		seqNum := uint32(i + 1)
		seen := false
		for _, flag := range msg.Flags {
			if flag == imap.SeenFlag {
				seen = true
				break
			}
		}
		if !seen {
			return seqNum
		}
	}
	return 0
}

func (mbox *MailBox_impl) flags() []string {
	flagsMap := make(map[string]bool)
	messages := mbox.getMessages()
	for _, msg := range messages {
		for _, f := range msg.Flags {
			if !flagsMap[f] {
				flagsMap[f] = true
			}
		}
	}
	var flags []string
	for f := range flagsMap {
		flags = append(flags, f)
	}
	return flags
}

// Status returns this mailbox status. Public prototype preserved.
func (mbox *MailBox_impl) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	status := imap.NewMailboxStatus(mbox.name, items)
	status.Flags = mbox.flags()
	status.PermanentFlags = []string{"\\*"}
	status.UnseenSeqNum = mbox.unseenSeqNum()

	for _, name := range items {
		switch name {
		case imap.StatusMessages:
			status.Messages = uint32(len(mbox.getMessages()))
		case imap.StatusUidNext:
			status.UidNext = mbox.uidNext()
		case imap.StatusUidValidity:
			status.UidValidity = 1
		case imap.StatusRecent:
			status.Recent = 0 // TODO
		case imap.StatusUnseen:
			status.Unseen = 0 // TODO
		}
	}

	return status, nil
}

// SetSubscribed marks the mailbox as (un)subscribed. Public prototype preserved.
func (mbox *MailBox_impl) SetSubscribed(subscribed bool) error {
	mbox.subscribed = subscribed
	return nil
}

// Check performs a mailbox checkpoint (no-op here). Public prototype preserved.
func (mbox *MailBox_impl) Check() error {
	return nil
}

// ListMessages streams messages that match seqSet into ch. Public prototype preserved.
func (mbox *MailBox_impl) ListMessages(uid bool, seqSet *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	defer close(ch)
	messages := mbox.getMessages()

	for i, msg := range messages {
		seqNum := uint32(i + 1)

		var id uint32
		if uid {
			id = msg.Uid
		} else {
			id = seqNum
		}
		if !seqSet.Contains(id) {
			continue
		}

		m, err := msg.Fetch(seqNum, items)
		if err != nil {
			logger.Warn("ListMessages: fetch failed", "uid_mode", uid, "id", id, "err", err)
			continue
		}

		ch <- m
	}

	return nil
}

// SearchMessages returns IDs of messages that match criteria. Public prototype preserved.
func (mbox *MailBox_impl) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	var ids []uint32
	messages := mbox.getMessages()

	for i, msg := range messages {
		seqNum := uint32(i + 1)

		ok, err := msg.Match(seqNum, criteria)
		if err != nil || !ok {
			if err != nil {
				logger.Warn("SearchMessages: match failed", "seq", seqNum, "uid", msg.Uid, "err", err)
			}
			continue
		}

		if uid {
			ids = append(ids, msg.Uid)
		} else {
			ids = append(ids, seqNum)
		}
	}
	return ids, nil
}

// CreateMessage appends a new message to the mailbox. Public prototype preserved.
func (mbox *MailBox_impl) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	if date.IsZero() {
		date = time.Now()
	}

	b, err := io.ReadAll(body)
	if err != nil {
		logger.Error("CreateMessage: read body failed", "user", mbox.user, "mailbox", mbox.name, "err", err)
		return err
	}

	if err := saveMessage(mbox.user, mbox.name, b, flags, date); err != nil {
		logger.Error("CreateMessage: save failed", "user", mbox.user, "mailbox", mbox.name, "err", err)
		return err
	}
	return nil
}

// UpdateMessagesFlags updates message flags for a set. Public prototype preserved.
func (mbox *MailBox_impl) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, op imap.FlagsOp, flags []string) error {
	messages := mbox.getMessages()

	for i, msg := range messages {
		var id uint32
		if uid {
			id = msg.Uid
		} else {
			id = uint32(i + 1)
		}
		if !seqset.Contains(id) {
			continue
		}

		msg.Flags = backendutil.UpdateFlags(msg.Flags, op, flags)

		// Persist updated flags
		connectionId := mbox.user + "_db"
		jsonStr, _ := Utility.ToJson(msg.Flags)
		if err := Store.UpdateOne(
			connectionId, connectionId, mbox.name,
			`{"Uid":`+Utility.ToString(msg.Uid)+`}`,
			`{ "$set":{"Flags":`+jsonStr+`}}`,
			"",
		); err != nil {
			logger.Error("UpdateMessagesFlags: persist failed",
				"user", mbox.user, "mailbox", mbox.name, "uid", msg.Uid, "err", err)
			return err
		}
	}
	return nil
}

// CopyMessages copies selected messages to another mailbox. Public prototype preserved.
func (mbox *MailBox_impl) CopyMessages(uid bool, seqset *imap.SeqSet, destName string) error {
	dest, err := getMailBox(mbox.user, destName)
	if err != nil {
		logger.Error("CopyMessages: destination mailbox not found",
			"user", mbox.user, "src", mbox.name, "dest", destName, "err", err)
		return err
	}

	messages := mbox.getMessages()
	for i, msg := range messages {
		var id uint32
		if uid {
			id = msg.Uid
		} else {
			id = uint32(i + 1)
		}
		if !seqset.Contains(id) {
			continue
		}
		if err := saveMessage(dest.user, dest.name, msg.Body, msg.Flags, time.Now()); err != nil {
			logger.Error("CopyMessages: save to dest failed",
				"user", mbox.user, "src", mbox.name, "dest", destName, "uid", msg.Uid, "err", err)
			return err
		}
	}

	return nil
}

// Expunge removes all \Deleted messages permanently. Public prototype preserved.
func (mbox *MailBox_impl) Expunge() error {
	messages := mbox.getMessages()

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		deleted := false
		for _, flag := range msg.Flags {
			if flag == imap.DeletedFlag {
				deleted = true
				break
			}
		}
		if deleted {
			connectionId := mbox.user + "_db"
			if err := Store.DeleteOne(
				connectionId, connectionId, mbox.name,
				`{"Uid":`+Utility.ToString(msg.Uid)+`}`,
				"",
			); err != nil {
				logger.Error("Expunge: delete failed",
					"user", mbox.user, "mailbox", mbox.name, "uid", msg.Uid, "err", err)
				return err
			}
		}
	}
	return nil
}
