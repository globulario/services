package imap

import (
	"errors"

	// "io/ioutil"
	"io/ioutil"
	"log"
	"time"

	"encoding/json"

	b64 "encoding/base64"

	"github.com/davecourtois/Utility"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend/backendutil"
)

type MailBox_impl struct {
	name       string
	user       string
	subscribed bool
}

func NewMailBox(user string, name string) *MailBox_impl {
	log.Println("NewMailBox call with name ", name, " and user ", user)
	box := new(MailBox_impl)
	box.name = name
	box.user = user

	info := new(imap.MailboxInfo)
	info.Name = name
	info.Delimiter = "/"
	jsonStr, err := Utility.ToJson(info)
	if err != nil {
		log.Println(err)
		return nil
	}
	connectionId := user + "_db"

	// Here I will retreive the info from the database.
	query := `{"Name":"` + name + `"}`
	count, err := Store.Count(connectionId, connectionId, "MailBoxes", query, "")

	if err != nil || count < 1 {

		log.Println(info)
		_, err = Store.InsertOne(connectionId, connectionId, "MailBoxes", jsonStr, "")

		if err != nil {
			log.Println("fail to create mail box!")
		}
	}
	log.Println("succeed to create new mail box ", name)
	return box
}

func getMailBox(user string, name string) (*MailBox_impl, error) {
	log.Println("getMailBox ", name, " for user ", name)
	box := new(MailBox_impl)
	box.name = name
	box.user = user

	info := new(imap.MailboxInfo)
	info.Name = name
	info.Delimiter = "/"

	connectionId := user + "_db"

	// Here I will retreive the info from the database.
	query := `{"Name":"` + name + `"}`
	count, err := Store.Count(connectionId, connectionId, "MailBoxes", query, "")
	if err != nil || count < 1 {
		return nil, errors.New("No mail box found with name " + name)
	}
	log.Println("getMailBox ", name, " for user ", name, " succeed!")
	return box, nil
}

// Name returns this mailbox name.
func (mbox *MailBox_impl) Name() string {
	return mbox.name
}

// Info returns this mailbox info.
func (mbox *MailBox_impl) Info() (*imap.MailboxInfo, error) {
	log.Println("get mail box info for user ", mbox.user, " name ", mbox.name)
	// TODO Get box info from the server.
	connectionId := mbox.user + "_db"
	query := `{"Name":"` + mbox.name + `"}`
	jsonStr, err := Store.FindOne(connectionId, connectionId, "MailBoxes", query, "")
	if err != nil {
		log.Println(err)
		return nil, err
	}

	info_ := make(map[string]interface{})
	err = json.Unmarshal([]byte(jsonStr), &info_)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// Now I will insert the message into the inbox of the user.
	info := new(imap.MailboxInfo)
	info.Name = info_["Name"].(string)
	info.Delimiter = info_["Delimiter"].(string)
	if info_["Attributes"] != nil {
		log.Println("attributes ", info_["Attributes"])
	}
	log.Println("get mail box info for user ", mbox.user, " succeed!")
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

// Return the list of message from the bakend.
func (mbox *MailBox_impl) getMessages() []*Message {
	log.Println("---> getMessages")
	messages := make([]*Message, 0)
	connectionId := mbox.user + "_db"

	// Get the message from the mailbox.
	data, err := Store.Find(connectionId, connectionId, mbox.Name(), "{}", "")
	if err != nil {
		return messages
	}

	// return the messages.
	for i := 0; i < len(data); i++ {
		msg := data[i].(map[string]interface{})
		m := new(Message)
		m.Uid = uint32(msg["Uid"].(float64)) // set the actual index
		data, err := b64.StdEncoding.DecodeString(msg["Body"].(string))
		if err == nil {
			m.Body = data
		}
		flags := msg["Flags"].([]interface{})
		m.Flags = make([]string, 0)
		for j := 0; j < len(flags); j++ {
			if len(flags[j].(string)) > 0 {
				m.Flags = append(m.Flags, flags[j].(string))
			}
		}

		m.Size = uint32(msg["Size"].(float64))

		layout := "2020-11-02T01:45:47.764336457Z"
		m.Date, _ = time.Parse(layout, msg["Date"].(string))

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

// Status returns this mailbox status. The fields Name, Flags, PermanentFlags
// and UnseenSeqNum in the returned MailboxStatus must be always populated.
// This function does not affect the state of any messages in the mailbox. See
// RFC 3501 section 6.3.10 for a list of items that can be requested.
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

// SetSubscribed adds or removes the mailbox to the server's set of "active"
// or "subscribed" mailboxes.
func (mbox *MailBox_impl) SetSubscribed(subscribed bool) error {
	mbox.subscribed = subscribed
	return nil
}

// Check requests a checkpoint of the currently selected mailbox. A checkpoint
// refers to any implementation-dependent housekeeping associated with the
// mailbox (e.g., resolving the server's in-memory state of the mailbox with
// the state on its disk). A checkpoint MAY take a non-instantaneous amount of
// real time to complete. If a server implementation has no such housekeeping
// considerations, CHECK is equivalent to NOOP.
func (mbox *MailBox_impl) Check() error {
	return nil
}

// ListMessages returns a list of messages. seqset must be interpreted as UIDs
// if uid is set to true and as message sequence numbers otherwise. See RFC
// 3501 section 6.4.5 for a list of items that can be requested.
//
// Messages must be sent to ch. When the function returns, ch must be closed.
func (mbox *MailBox_impl) ListMessages(uid bool, seqSet *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	log.Println("---> ListMessages")
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
			continue
		}

		ch <- m
	}

	return nil
}

// SearchMessages searches messages. The returned list must contain UIDs if
// uid is set to true, or sequence numbers otherwise.
func (mbox *MailBox_impl) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {

	var ids []uint32
	messages := mbox.getMessages()
	for i, msg := range messages {
		seqNum := uint32(i + 1)

		ok, err := msg.Match(seqNum, criteria)
		if err != nil || !ok {
			continue
		}

		var id uint32
		if uid {
			id = msg.Uid
		} else {
			id = seqNum
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// CreateMessage appends a new message to this mailbox. The \Recent flag will
// be added no matter flags is empty or not. If date is nil, the current time
// will be used.
//
// If the Backend implements Updater, it must notify the client immediately
// via a mailbox update.
func (mbox *MailBox_impl) CreateMessage(flags []string, date time.Time, body imap.Literal) error {

	if date.IsZero() {
		date = time.Now()
	}

	b, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}

	return saveMessage(mbox.user, mbox.name, b, flags, date)
}

// UpdateMessagesFlags alters flags for the specified message(s).
//
// If the Backend implements Updater, it must notify the client immediately
// via a message update.
func (mbox *MailBox_impl) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, op imap.FlagsOp, flags []string) error {
	log.Println("-----> flags ", flags)
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

		// Here I will save the message into the database.
		connectionId := mbox.user + "_db"
		jsonStr, _ := Utility.ToJson(msg.Flags)

		err := Store.UpdateOne(connectionId, connectionId, mbox.name, `{"Uid":`+Utility.ToString(msg.Uid)+`}`, `{ "$set":{"Flags":`+jsonStr+`}}`, "")
		if err != nil {
			log.Println(err)
			return err
		}

	}
	return nil
}

// CopyMessages copies the specified message(s) to the end of the specified
// destination mailbox. The flags and internal date of the message(s) SHOULD
// be preserved, and the Recent flag SHOULD be set, in the copy.
//
// If the destination mailbox does not exist, a server SHOULD return an error.
// It SHOULD NOT automatically create the mailbox.
//
// If the Backend implements Updater, it must notify the client immediately
// via a mailbox update.
func (mbox *MailBox_impl) CopyMessages(uid bool, seqset *imap.SeqSet, destName string) error {

	dest, err := getMailBox(mbox.user, destName)
	if err != nil {
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

		// save the message in the backend.
		saveMessage(dest.user, dest.name, msg.Body, msg.Flags, time.Now())
	}

	return nil
}

// Expunge permanently removes all messages that have the \Deleted flag set
// from the currently selected mailbox.
//
// If the Backend implements Updater, it must notify the client immediately
// via an expunge update.
func (mbox *MailBox_impl) Expunge() error {
	log.Println("=--------> expunge!")
	messages := mbox.getMessages()

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		log.Println("----> 415")
		deleted := false
		for _, flag := range msg.Flags {
			if flag == imap.DeletedFlag {
				deleted = true
				break
			}
		}
		log.Println("----> 424")
		if deleted {
			log.Println("----> 426")
			// mbox.Messages = append(mbox.Messages[:i], mbox.Messages[i+1:]...)
			connectionId := mbox.user + "_db"
			err := Store.DeleteOne(connectionId, connectionId, mbox.name, `{"Uid":`+Utility.ToString(msg.Uid)+`}`, "")
			if err != nil {
				log.Println("--------> fail to delete message from message box ", mbox.name, err)
				return err
			}
		}
	}

	return nil
}
