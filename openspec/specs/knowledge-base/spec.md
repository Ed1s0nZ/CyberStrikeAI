# Knowledge Base

## Purpose
Define how a filesystem-backed Markdown corpus becomes a searchable, indexable, and auditable knowledge service for operators and agents.

## Requirements
### Requirement: Discoverable Markdown corpus
The system SHALL discover and normalize Markdown knowledge items from the configured knowledge root.

#### Scenario: Scanner walks the corpus
- **WHEN** the knowledge scanner traverses the configured base path
- **THEN** Markdown files are converted into durable knowledge items with derived metadata

### Requirement: Searchable indexed retrieval
The system SHALL provide indexed retrieval over discovered knowledge items.

#### Scenario: Search is requested after indexing
- **WHEN** a caller issues a retrieval request against indexed knowledge
- **THEN** the system returns results from the retrieval layer and may log the retrieval context for audit

### Requirement: Incremental index maintenance
The system SHALL prefer incremental indexing when a valid index already exists.

#### Scenario: Existing corpus item changes
- **WHEN** a previously discovered Markdown item changes content
- **THEN** the item is marked for reindex rather than forcing a full corpus rebuild

### Requirement: Full rebuild fallback
The system SHALL support full rebuild when no valid index exists.

#### Scenario: Knowledge index is absent
- **WHEN** the system determines that no valid retrieval index exists
- **THEN** the subsystem performs or offers a full rebuild path for searchable readiness

### Requirement: Retrieval audit logging
The system SHALL record retrieval logs when conversation/message context is available.

#### Scenario: Agent performs a knowledge search during conversation
- **WHEN** retrieval is executed in a conversation-aware context
- **THEN** the system records the query and retrieved item references for later audit

### Requirement: Failure isolation during scan and index
Single-file or repeated index failures SHALL not silently corrupt the whole corpus state.

#### Scenario: One Markdown file cannot be read
- **WHEN** the scanner encounters an unreadable knowledge file
- **THEN** that file is skipped with visible failure while the remainder of the corpus continues processing

## Overview
The knowledge base domain converts a filesystem-backed Markdown corpus into a searchable retrieval service for operators and agents. It supports corpus scanning, indexing, retrieval, statistics, and retrieval audit logging, with optional separation of the knowledge database from the primary conversation database.

## Capabilities
### Capability 1: Filesystem-Backed Corpus Discovery
- The system must scan the configured knowledge root for Markdown source files.
- The system must derive knowledge item metadata such as category, title, and source path from the corpus layout.

### Capability 2: Index Lifecycle Management
- The system must build retrieval indexes for searchable knowledge items.
- The system must prefer incremental indexing for changed items when a valid index already exists.
- The system must support full rebuild when no valid index is present.

### Capability 3: Search And Retrieval APIs
- The system must expose APIs for category listing, item listing, item detail, search, index status, and statistics.
- The system must register agent-facing retrieval tools over the MCP surface.

### Capability 4: Retrieval Auditability
- The system must record retrieval logs that can be linked to conversations and messages when the caller context is available.

## Interfaces
- `GET /api/knowledge/categories`
- `GET /api/knowledge/items`
- `GET /api/knowledge/items/:id`
- `POST /api/knowledge/items`
- `PUT /api/knowledge/items/:id`
- `DELETE /api/knowledge/items/:id`
- `GET /api/knowledge/index-status`
- `POST /api/knowledge/index`
- `POST /api/knowledge/scan`
- `POST /api/knowledge/search`
- `GET /api/knowledge/retrieval-logs`
- `DELETE /api/knowledge/retrieval-logs/:id`
- `GET /api/knowledge/stats`

## State Machine
- Knowledge subsystem lifecycle:
  - `Disabled`
  - `Initialized`
  - `Scanning`
  - `Indexed`
  - `Searching`
  - `Reindexing`
  - `Degraded`
- Item lifecycle:
  - `Discovered`
  - `Stored`
  - `Indexed`
  - `Updated`
  - `Deleted`

## Data Flow
1. The scanner walks the configured base path and discovers Markdown files.
2. Files are normalized into knowledge items with category, title, path, and content.
3. New or changed items are marked for indexing.
4. The embedding/indexing layer builds or updates search vectors and retrieval metadata.
5. Search requests query the retrieval layer and optionally log retrieved items against the calling conversation/message.
6. Agents consume the same retrieval capability through registered MCP tools.

## Constraints
- Markdown is the canonical corpus format for filesystem-backed knowledge ingestion.
- The first directory segment under the knowledge root determines the item's category or risk type.
- If embedding credentials are omitted for the knowledge subsystem, the platform may inherit them from the main model configuration.
- Incremental indexing is preferred when an index already exists; full rebuild is reserved for missing or invalid index state.
- Retrieval logging is audit metadata and must not modify the source knowledge item itself.
- Knowledge data may live in a separate database, but the interface contract must remain unchanged.

## Failure Handling
- If the knowledge subsystem is disabled, its APIs and tools are unavailable rather than returning misleading empty success.
- File-level scan errors should be logged and skipped so one bad file does not poison the entire corpus.
- Repeated indexing failures should stop the current incremental pass to avoid runaway error loops.
- If retrieval logs cannot be written, the search result may still be returned, but the missing audit trail must be treated as degraded behavior.
- If index state is absent, the system should rebuild rather than serving semantically stale search guarantees.
