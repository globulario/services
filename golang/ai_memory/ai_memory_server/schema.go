package main

import "fmt"

// ScyllaDB schema for the AI Memory service.
//
// Keyspace: ai_memory (SimpleStrategy, RF=1 for single-node; adjust for cluster)
//
// Run these CQL statements to initialize the schema:
//
//   CREATE KEYSPACE IF NOT EXISTS ai_memory
//     WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};
//
//   -- Main memory table, partitioned by project for efficient per-project queries.
//   CREATE TABLE IF NOT EXISTS ai_memory.memories (
//       id              text,
//       project         text,
//       type            text,
//       tags            set<text>,
//       title           text,
//       content         text,
//       created_at      bigint,
//       updated_at      bigint,
//       agent_id        text,
//       conversation_id text,
//       cluster_id      text,
//       PRIMARY KEY ((project), type, created_at, id)
//   ) WITH CLUSTERING ORDER BY (type ASC, created_at DESC, id ASC);
//
//   -- Session table for conversation continuity.
//   CREATE TABLE IF NOT EXISTS ai_memory.sessions (
//       id               text,
//       project          text,
//       topic            text,
//       summary          text,
//       decisions        list<text>,
//       open_questions   list<text>,
//       related_memories list<text>,
//       created_at       bigint,
//       agent_id         text,
//       cluster_id       text,
//       PRIMARY KEY ((project), created_at, id)
//   ) WITH CLUSTERING ORDER BY (created_at DESC, id ASC);
//
//   -- Secondary index on tags for tag-based queries.
//   CREATE INDEX IF NOT EXISTS idx_memories_tags
//     ON ai_memory.memories (tags);
//
// Notes:
//   - TTL is applied per-row at INSERT/UPDATE time using ScyllaDB's USING TTL.
//   - Text search is done via ALLOW FILTERING on title/content (sufficient for
//     the expected data volume; upgrade to Elasticsearch if volume grows).
//   - The partition key (project) ensures all memories for a project are co-located
//     for efficient range scans.

const keyspace = "ai_memory"

// createKeyspaceCQL returns the CQL for creating the ai_memory keyspace,
// adjusting the replication factor to the number of available ScyllaDB nodes.
func createKeyspaceCQL(rf int) string {
	return fmt.Sprintf(`
CREATE KEYSPACE IF NOT EXISTS ai_memory
  WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}
`, rf)
}

const createMemoriesTableCQL = `
CREATE TABLE IF NOT EXISTS ai_memory.memories (
    id              text,
    project         text,
    type            text,
    tags            set<text>,
    title           text,
    content         text,
    created_at      bigint,
    updated_at      bigint,
    agent_id        text,
    conversation_id text,
    cluster_id      text,
    metadata        map<text, text>,
    related_ids     list<text>,
    reference_count int,
    PRIMARY KEY ((project), type, created_at, id)
) WITH CLUSTERING ORDER BY (type ASC, created_at DESC, id ASC)
`

const createSessionsTableCQL = `
CREATE TABLE IF NOT EXISTS ai_memory.sessions (
    id               text,
    project          text,
    topic            text,
    summary          text,
    decisions        list<text>,
    open_questions   list<text>,
    related_memories list<text>,
    created_at       bigint,
    agent_id         text,
    cluster_id       text,
    PRIMARY KEY ((project), created_at, id)
) WITH CLUSTERING ORDER BY (created_at DESC, id ASC)
`

const createTagsIndexCQL = `
CREATE INDEX IF NOT EXISTS idx_memories_tags
  ON ai_memory.memories (tags)
`
