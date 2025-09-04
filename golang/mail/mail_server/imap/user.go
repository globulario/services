package imap

import (
	"errors"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	Utility "github.com/globulario/utility"
)


////////////////////////////////////////////////////////////////////////////////
// The User implementation
////////////////////////////////////////////////////////////////////////////////

// User_impl implements go-imap's backend.User for a single authenticated user.
// It expects the persistence Store and related globals to be set by the service.
// Public prototypes are preserved for compatibility.
type User_impl struct {
	// info contains values loaded from the persistence layer (e.g., Mongo).
	info map[string]interface{}
}

// Username returns this user's username.
// Public prototype preserved.
func (user *User_impl) Username() string {
	if user.info == nil {
		logger.Warn("Username: user.info is nil")
		return ""
	}
	v, ok := user.info["name"].(string)
	if !ok {
		logger.Warn("Username: missing or invalid 'name' in user.info")
		return ""
	}
	return v
}

// ListMailboxes returns a list of mailboxes belonging to this user. If
// subscribed is true, only returns subscribed mailboxes.
// Public prototype preserved.
func (user *User_impl) ListMailboxes(subscribed bool) ([]backend.Mailbox, error) {
	username := user.Username()
	if username == "" {
		return nil, errors.New("ListMailboxes: empty username")
	}

	connectionId := username + "_db"

	// Fetch all mailbox metadata for this user.
	values, err := Store.Find(connectionId, connectionId, "MailBoxes", `{}`, ``)
	if err != nil {
		logger.Error("ListMailboxes: backend Find failed", "user", username, "err", err)
		return nil, err
	}

	boxes := make([]backend.Mailbox, 0, len(values)+1)

	for i := 0; i < len(values); i++ {
		val, ok := values[i].(map[string]interface{})
		if !ok {
			logger.Warn("ListMailboxes: invalid mailbox record", "user", username, "index", i)
			continue
		}
		name, ok := val["Name"].(string)
		if !ok || name == "" {
			logger.Warn("ListMailboxes: mailbox record missing Name", "user", username, "index", i)
			continue
		}
		box := NewMailBox(username, name)
		if box != nil {
			boxes = append(boxes, box)
		}
	}

	// Ensure at least INBOX exists by default.
	if len(values) == 0 {
		inbox := NewMailBox(username, "INBOX")
		if inbox != nil {
			boxes = append(boxes, inbox)
		}
	}

	// NOTE: 'subscribed' filtering is not persisted here; if you later store
	// subscription state, you can filter boxes based on that.
	return boxes, nil
}

// GetMailbox returns a mailbox. If it doesn't exist, it returns ErrNoSuchMailbox.
// Public prototype preserved.
func (user *User_impl) GetMailbox(name string) (backend.Mailbox, error) {
	username := user.Username()
	if username == "" {
		return nil, errors.New("GetMailbox: empty username")
	}
	if name == "" {
		return nil, errors.New("GetMailbox: empty mailbox name")
	}

	connectionId := username + "_db"
	query := `{"Name":"` + name + `"}`
	count, err := Store.Count(connectionId, connectionId, "MailBoxes", query, "")
	if err != nil || count < 1 {
		if err == nil {
			err = errors.New("no such mailbox")
		}
		logger.Warn("GetMailbox: mailbox not found", "user", username, "name", name, "err", err)
		return nil, errors.New("No mail box found with name " + name)
	}

	return NewMailBox(username, name), nil
}

// CreateMailbox creates a new mailbox.
// Public prototype preserved.
func (user *User_impl) CreateMailbox(name string) error {
	username := user.Username()
	if username == "" {
		return errors.New("CreateMailbox: empty username")
	}
	if name == "" {
		return errors.New("CreateMailbox: empty mailbox name")
	}

	info := new(imap.MailboxInfo)
	info.Name = name
	info.Delimiter = "/"

	jsonStr, err := Utility.ToJson(info)
	if err != nil {
		logger.Error("CreateMailbox: marshal MailboxInfo failed", "user", username, "name", name, "err", err)
		return err
	}

	connectionId := username + "_db"
	if _, err := Store.InsertOne(connectionId, connectionId, "MailBoxes", jsonStr, ""); err != nil {
		logger.Error("CreateMailbox: insert failed", "user", username, "name", name, "err", err)
		return err
	}

	logger.Info("CreateMailbox: created", "user", username, "name", name)
	return err
}

// DeleteMailbox permanently removes the mailbox with the given name.
// Public prototype preserved.
func (user *User_impl) DeleteMailbox(name string) error {
	username := user.Username()
	if username == "" {
		return errors.New("DeleteMailbox: empty username")
	}
	if name == "" {
		return errors.New("DeleteMailbox: empty mailbox name")
	}

	connectionId := username + "_db"

	// Remove mailbox metadata entry
	if err := Store.DeleteOne(connectionId, connectionId, "MailBoxes", `{"Name":"`+name+`"}`, ""); err != nil {
		logger.Error("DeleteMailbox: delete metadata failed", "user", username, "name", name, "err", err)
		return err
	}

	// Drop the collection holding message documents
	if err := Store.DeleteCollection(connectionId, connectionId, name); err != nil {
		logger.Error("DeleteMailbox: delete collection failed", "user", username, "name", name, "err", err)
		return err
	}

	logger.Info("DeleteMailbox: removed", "user", username, "name", name)
	return nil
}

// RenameMailbox changes the name of a mailbox, and renames its underlying collection.
// Public prototype preserved.
func (user *User_impl) RenameMailbox(existingName, newName string) error {
	username := user.Username()
	if username == "" {
		return errors.New("RenameMailbox: empty username")
	}
	if existingName == "" || newName == "" {
		return errors.New("RenameMailbox: missing existing or new name")
	}

	connectionId := username + "_db"

	// Update mailbox metadata document
	if err := Store.UpdateOne(
		connectionId, connectionId, "MailBoxes",
		`{"Name":"`+existingName+`"}`,
		`{"$set":{"Name":"`+newName+`"}}`,
		"",
	); err != nil {
		logger.Error("RenameMailbox: update metadata failed", "user", username, "from", existingName, "to", newName, "err", err)
		return err
	}

	// Rename the actual collection with admin privileges
	if err := renameCollection(connectionId, existingName, newName); err != nil {
		logger.Error("RenameMailbox: rename collection failed", "user", username, "from", existingName, "to", newName, "err", err)
		return err
	}

	logger.Info("RenameMailbox: renamed", "user", username, "from", existingName, "to", newName)
	return nil
}

// Logout is called when this User will no longer be used, likely because the
// client closed the connection.
// Public prototype preserved.
func (user *User_impl) Logout() error {
	// If you maintain per-user connections, you could disconnect here:
	// return Store.Disconnect(user.Username() + "_db")
	return nil
}
