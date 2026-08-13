package multiagent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const currentCairnStateVersion = 2

type Fact struct {
	ID            string    `yaml:"id"`
	Description   string    `yaml:"description"`
	CreatedAt     time.Time `yaml:"created_at"`
	SourceIntent  string    `yaml:"source_intent,omitempty"`
	SourceEventID string    `yaml:"source_event_id,omitempty"`
}

type Intent struct {
	ID          string   `yaml:"id"`
	From        []string `yaml:"from"`
	Description string   `yaml:"description"`
	Status      string   `yaml:"status"`
}

type Hint struct {
	ID        string    `yaml:"id"`
	Content   string    `yaml:"content"`
	Creator   string    `yaml:"creator"`
	CreatedAt time.Time `yaml:"created_at"`
}

type CairnState struct {
	Version   int       `yaml:"version,omitempty"`
	UpdatedAt time.Time `yaml:"updated_at,omitempty"`
	Facts     []Fact    `yaml:"facts"`
	Intents   []Intent  `yaml:"intents"`
	Hints     []Hint    `yaml:"hints"`
	Goal      string    `yaml:"goal"`
	Origin    string    `yaml:"origin"`
}

var cairnStateLocks sync.Map

func cairnStateLock(path string) *sync.Mutex {
	v, _ := cairnStateLocks.LoadOrStore(path, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func StateFilePath(stateDir, projectID, conversationID string) string {
	if stateDir == "" {
		stateDir = "state"
	}
	return filepath.Join(stateDir, projectID, fmt.Sprintf("%s-state.yaml", conversationID))
}

func normalizeCairnState(st *CairnState) {
	if st.Version <= 0 {
		st.Version = currentCairnStateVersion
	}
	if st.UpdatedAt.IsZero() {
		st.UpdatedAt = time.Now().UTC()
	}
	if st.Facts == nil {
		st.Facts = []Fact{}
	}
	if st.Intents == nil {
		st.Intents = []Intent{}
	}
	if st.Hints == nil {
		st.Hints = []Hint{}
	}
}

func loadCairnStateUnlocked(path string) (*CairnState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			st := &CairnState{Version: currentCairnStateVersion}
			normalizeCairnState(st)
			return st, nil
		}
		return nil, fmt.Errorf("cairn state read: %w", err)
	}
	var st CairnState
	if err := yaml.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("cairn state parse: %w", err)
	}
	normalizeCairnState(&st)
	return &st, nil
}

func LoadCairnState(path string) (*CairnState, error) {
	mu := cairnStateLock(path)
	mu.Lock()
	defer mu.Unlock()
	return loadCairnStateUnlocked(path)
}

func saveCairnStateUnlocked(path string, st *CairnState) error {
	if st == nil {
		return fmt.Errorf("cairn state is nil")
	}
	normalizeCairnState(st)
	st.Version = currentCairnStateVersion
	st.UpdatedAt = time.Now().UTC()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cairn state mkdir: %w", err)
	}
	data, err := yaml.Marshal(st)
	if err != nil {
		return fmt.Errorf("cairn state marshal: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".cairn-state-*.tmp")
	if err != nil {
		return fmt.Errorf("cairn state temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("cairn state chmod: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("cairn state write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cairn state close: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("cairn state rename: %w", err)
	}
	return nil
}

func SaveCairnState(path string, st *CairnState) error {
	mu := cairnStateLock(path)
	mu.Lock()
	defer mu.Unlock()
	return saveCairnStateUnlocked(path, st)
}

func UpdateCairnState(path string, fn func(*CairnState) error) (*CairnState, error) {
	if fn == nil {
		return nil, fmt.Errorf("cairn state update function is nil")
	}
	mu := cairnStateLock(path)
	mu.Lock()
	defer mu.Unlock()
	st, err := loadCairnStateUnlocked(path)
	if err != nil {
		return nil, err
	}
	if err := fn(st); err != nil {
		return nil, err
	}
	if err := saveCairnStateUnlocked(path, st); err != nil {
		return nil, err
	}
	return st, nil
}

func stableCairnID(prefix string, parts ...string) string {
	h := sha256.New()
	h.Write([]byte(prefix))
	for _, p := range parts {
		h.Write([]byte{0})
		h.Write([]byte(strings.TrimSpace(p)))
	}
	return prefix + "_" + hex.EncodeToString(h.Sum(nil))[:16]
}

func (st *CairnState) AddFact(description, sourceIntent string) Fact {
	f, _ := st.AddFactIdempotent(stableCairnID("event", sourceIntent, description), description, sourceIntent)
	return f
}

func (st *CairnState) AddFactIdempotent(eventID, description, sourceIntent string) (Fact, bool) {
	description = strings.TrimSpace(description)
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		eventID = stableCairnID("event", sourceIntent, description)
	}
	for _, existing := range st.Facts {
		if existing.SourceEventID == eventID {
			return existing, false
		}
	}
	f := Fact{
		ID:            stableCairnID("fact", eventID),
		Description:   description,
		CreatedAt:     time.Now().UTC(),
		SourceIntent:  strings.TrimSpace(sourceIntent),
		SourceEventID: eventID,
	}
	st.Facts = append(st.Facts, f)
	return f, true
}

func (st *CairnState) AddIntent(from []string, description string) Intent {
	description = strings.TrimSpace(description)
	id := stableCairnID("intent", strings.Join(from, ","), description)
	for i := range st.Intents {
		if st.Intents[i].ID == id {
			return st.Intents[i]
		}
	}
	it := Intent{ID: id, From: append([]string(nil), from...), Description: description, Status: "open"}
	st.Intents = append(st.Intents, it)
	return it
}

func (st *CairnState) AddHint(content, creator string) Hint {
	content = strings.TrimSpace(content)
	id := stableCairnID("hint", creator, content)
	for i := range st.Hints {
		if st.Hints[i].ID == id {
			return st.Hints[i]
		}
	}
	h := Hint{ID: id, Content: content, Creator: strings.TrimSpace(creator), CreatedAt: time.Now().UTC()}
	st.Hints = append(st.Hints, h)
	return h
}

func (st *CairnState) OpenIntents() []Intent {
	var out []Intent
	for _, it := range st.Intents {
		if it.Status == "open" {
			out = append(out, it)
		}
	}
	return out
}

func (st *CairnState) MarkIntentDone(intentID string) bool {
	for i := range st.Intents {
		if st.Intents[i].ID == intentID {
			st.Intents[i].Status = "done"
			return true
		}
	}
	return false
}

func (st *CairnState) MarkIntentDoneByDescription(description string) bool {
	description = strings.TrimSpace(description)
	for i := range st.Intents {
		if st.Intents[i].Description == description && st.Intents[i].Status != "done" {
			st.Intents[i].Status = "done"
			return true
		}
	}
	return false
}

func (st *CairnState) MarkIntentInProgress(intentID string) bool {
	for i := range st.Intents {
		if st.Intents[i].ID == intentID {
			st.Intents[i].Status = "in_progress"
			return true
		}
	}
	return false
}

func (st *CairnState) DropIntent(intentID string) bool {
	for i := range st.Intents {
		if st.Intents[i].ID == intentID {
			st.Intents[i].Status = "dropped"
			return true
		}
	}
	return false
}

func (st *CairnState) FactIDs() []string {
	ids := make([]string, len(st.Facts))
	for i, f := range st.Facts {
		ids[i] = f.ID
	}
	return ids
}

func (st *CairnState) ToYAML() (string, error) {
	data, err := yaml.Marshal(st)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
