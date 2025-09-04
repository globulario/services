package imap

import (
	"bufio"
	"bytes"
	"io"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend/backendutil"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/textproto"
)


// -----------------------------------------------------------------------------
// Message
// -----------------------------------------------------------------------------

// Message represents a single stored email with minimal metadata required by
// the IMAP backend.
// Public prototype preserved.
type Message struct {
	Uid   uint32
	Date  time.Time
	Size  uint32
	Flags []string
	Body  []byte
}

// entity parses the raw Body into a MIME entity.
func (m *Message) entity() (*message.Entity, error) {
	return message.Read(bytes.NewReader(m.Body))
}

// headerAndBody returns the parsed header and a reader positioned on the body.
func (m *Message) headerAndBody() (textproto.Header, io.Reader, error) {
	body := bufio.NewReader(bytes.NewReader(m.Body))
	hdr, err := textproto.ReadHeader(body)
	return hdr, body, err
}

// Fetch materializes an imap.Message for the requested items and sequence number.
// Public prototype preserved.
func (m *Message) Fetch(seqNum uint32, items []imap.FetchItem) (*imap.Message, error) {
	fetched := imap.NewMessage(seqNum, items)

	for _, item := range items {
		switch item {
		case imap.FetchEnvelope:
			hdr, _, err := m.headerAndBody()
			if err != nil {
				logger.Warn("message.Fetch: header parse failed for envelope", "uid", m.Uid, "err", err)
				break
			}
			env, err := backendutil.FetchEnvelope(hdr)
			if err != nil {
				logger.Warn("message.Fetch: envelope build failed", "uid", m.Uid, "err", err)
				break
			}
			fetched.Envelope = env

		case imap.FetchBody, imap.FetchBodyStructure:
			hdr, body, err := m.headerAndBody()
			if err != nil {
				logger.Warn("message.Fetch: header parse failed for body", "uid", m.Uid, "err", err)
				break
			}
			bs, err := backendutil.FetchBodyStructure(hdr, body, item == imap.FetchBodyStructure)
			if err != nil {
				logger.Warn("message.Fetch: body structure build failed", "uid", m.Uid, "err", err)
				break
			}
			fetched.BodyStructure = bs

		case imap.FetchFlags:
			fetched.Flags = m.Flags

		case imap.FetchInternalDate:
			fetched.InternalDate = m.Date

		case imap.FetchRFC822Size:
			fetched.Size = m.Size

		case imap.FetchUid:
			fetched.Uid = m.Uid

		default:
			// Handle BODY[...] sections
			section, err := imap.ParseBodySectionName(item)
			if err != nil {
				logger.Warn("message.Fetch: invalid body section", "uid", m.Uid, "item", string(item), "err", err)
				break
			}

			body := bufio.NewReader(bytes.NewReader(m.Body))
			hdr, err := textproto.ReadHeader(body)
			if err != nil {
				logger.Warn("message.Fetch: header parse failed for section", "uid", m.Uid, "section", section, "err", err)
				return nil, err
			}

			l, err := backendutil.FetchBodySection(hdr, body, section)
			if err != nil {
				logger.Warn("message.Fetch: section fetch failed", "uid", m.Uid, "section", section, "err", err)
				break
			}
			fetched.Body[section] = l
		}
	}

	return fetched, nil
}

// Match returns whether the message satisfies the search criteria.
// Public prototype preserved.
func (m *Message) Match(seqNum uint32, c *imap.SearchCriteria) (bool, error) {
	e, err := m.entity()
	if err != nil {
		// Non-fatal: log and let backendutil decide using available fields.
		logger.Warn("message.Match: entity parse failed", "uid", m.Uid, "err", err)
	}
	return backendutil.Match(e, seqNum, m.Uid, m.Date, m.Flags, c)
}
