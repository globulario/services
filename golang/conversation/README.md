# Conversation Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Conversation Service provides real-time messaging and group conversation capabilities.

## Overview

This service enables building chat applications with support for multi-participant conversations, message threading, invitation workflows, and engagement tracking.

## Features

- **Real-Time Messaging** - Bidirectional streaming
- **Group Conversations** - Multi-participant chats
- **Message Threading** - Reply chains
- **Invitation System** - Invite/accept workflow
- **Engagement Tracking** - Likes, read receipts
- **Message Search** - Find messages by keyword

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Conversation Service                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  Message Router                            │ │
│  │                                                            │ │
│  │  Sender ──▶ Conversation ──▶ [Participant1, Participant2] │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                Connection Manager                          │ │
│  │                                                            │ │
│  │  User1 ──┬── Conversation A                               │ │
│  │          └── Conversation B                               │ │
│  │                                                            │ │
│  │  User2 ──┬── Conversation A                               │ │
│  │          └── Conversation C                               │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                Invitation Manager                          │ │
│  │                                                            │ │
│  │  Send ──▶ Pending ──▶ Accept/Decline ──▶ Join/Reject     │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Conversation Management

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateConversation` | Start new conversation | `name`, `participants[]` |
| `JoinConversation` | Join existing conversation | `conversationId`, `userId` |
| `LeaveConversation` | Leave conversation | `conversationId`, `userId` |

### Messaging

| Method | Description | Parameters |
|--------|-------------|------------|
| `Connect` | Open message stream | `conversationId` |
| `SendMessage` | Send message | `conversationId`, `message` |
| `DeleteMessage` | Remove message | `messageId` |
| `FindMessages` | Search messages | `keywords` |

### Invitations

| Method | Description | Parameters |
|--------|-------------|------------|
| `SendInvitation` | Invite user | `conversationId`, `userId` |
| `AcceptInvitation` | Accept invite | `invitationId` |
| `DeclineInvitation` | Decline invite | `invitationId` |

### Engagement

| Method | Description | Parameters |
|--------|-------------|------------|
| `LikeMessage` | Like a message | `messageId`, `userId` |
| `DislikeMessage` | Unlike a message | `messageId`, `userId` |
| `SetMessageRead` | Mark as read | `messageId`, `userId` |

## Usage Examples

### Go Client

```go
import (
    conv "github.com/globulario/services/golang/conversation/conversation_client"
)

client, _ := conv.NewConversationService_Client("localhost:10113", "conversation.ConversationService")
defer client.Close()

// Create conversation
conversation, err := client.CreateConversation("Project Chat", []string{"user-1", "user-2"})

// Connect to message stream
stream, err := client.Connect(conversation.Id)

// Send message
message := &convpb.Message{
    Id:        "msg-1",
    Author:    "user-1",
    Content:   "Hello everyone!",
    Timestamp: time.Now(),
}
err = client.SendMessage(conversation.Id, message)

// Receive messages
go func() {
    for {
        msg, err := stream.Recv()
        if err != nil {
            break
        }
        fmt.Printf("[%s] %s: %s\n",
            msg.Timestamp.Format("15:04"),
            msg.Author,
            msg.Content)
    }
}()

// Reply to message
reply := &convpb.Message{
    Id:        "msg-2",
    Author:    "user-2",
    Content:   "Hi! How's everyone doing?",
    ReplyTo:   "msg-1",
    Timestamp: time.Now(),
}
err = client.SendMessage(conversation.Id, reply)

// Like a message
err = client.LikeMessage("msg-1", "user-2")

// Mark as read
err = client.SetMessageRead("msg-1", "user-2")
```

### Invitation Flow

```go
// Invite user to conversation
err := client.SendInvitation(conversation.Id, "user-3")

// User-3 accepts invitation
err = client.AcceptInvitation(invitationId)

// User-3 can now join
err = client.JoinConversation(conversation.Id, "user-3")
```

## Configuration

### Configuration File

```json
{
  "port": 10113,
  "maxParticipants": 100,
  "maxMessageSize": "64KB",
  "messageRetention": "90d",
  "enableTypingIndicators": true
}
```

## Dependencies

- [Persistence Service](../persistence/README.md) - Message storage
- [Event Service](../event/README.md) - Real-time updates

---

[Back to Services Overview](../README.md)
