package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/ingatan/internal/domain"
)

// --- Mock implementations ---

type mockRepo struct {
	memories   map[string]*domain.Memory // key: store+"/"+id
	saveErr    error
	getErr     error
	updateErr  error
	deleteErr  error
	listErr    error
	listResult []*domain.Memory
	listTotal  int
}

func newMockRepo() *mockRepo {
	return &mockRepo{memories: make(map[string]*domain.Memory)}
}

func (m *mockRepo) key(store, id string) string { return store + "/" + id }

func (m *mockRepo) Save(_ context.Context, mem *domain.Memory) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.memories[m.key(mem.Store, mem.ID)] = mem
	return nil
}

func (m *mockRepo) Get(_ context.Context, store, id string) (*domain.Memory, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	mem, ok := m.memories[m.key(store, id)]
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeNotFound, "not found")
	}
	return mem, nil
}

func (m *mockRepo) Update(_ context.Context, mem *domain.Memory) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.memories[m.key(mem.Store, mem.ID)] = mem
	return nil
}

func (m *mockRepo) Delete(_ context.Context, store, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.memories, m.key(store, id))
	return nil
}

func (m *mockRepo) List(_ context.Context, _ string, _ Filter, _, _ int) ([]*domain.Memory, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.listResult, m.listTotal, nil
}

// mockChunkRepo records calls to verify chunk lifecycle.
type mockChunkRepo struct {
	chunks        map[string][]*domain.MemoryChunk // key: store+"/"+memoryID
	saveCalledN   int
	deleteCalledN int
	saveShouldErr error
}

func newMockChunkRepo() *mockChunkRepo {
	return &mockChunkRepo{chunks: make(map[string][]*domain.MemoryChunk)}
}

func (m *mockChunkRepo) key(store, memoryID string) string { return store + "/" + memoryID }

func (m *mockChunkRepo) SaveChunks(_ context.Context, store, memoryID string, chunks []*domain.MemoryChunk) error {
	if m.saveShouldErr != nil {
		return m.saveShouldErr
	}
	m.saveCalledN++
	m.chunks[m.key(store, memoryID)] = chunks
	return nil
}

func (m *mockChunkRepo) GetChunks(_ context.Context, store, memoryID string) ([]*domain.MemoryChunk, error) {
	return m.chunks[m.key(store, memoryID)], nil
}

func (m *mockChunkRepo) GetChunkByID(_ context.Context, _, chunkID string) (*domain.MemoryChunk, error) {
	for _, chunks := range m.chunks {
		for _, c := range chunks {
			if c.ChunkID == chunkID {
				return c, nil
			}
		}
	}
	return nil, domain.NewAppError(domain.ErrCodeNotFound, "chunk not found")
}

func (m *mockChunkRepo) DeleteChunks(_ context.Context, store, memoryID string) error {
	m.deleteCalledN++
	delete(m.chunks, m.key(store, memoryID))
	return nil
}

// mockChunker splits on every 50 chars for determinism.
type mockChunker struct {
	returnErr error
	results   []string
}

func (m *mockChunker) Chunk(content string) ([]string, error) {
	if m.returnErr != nil {
		return nil, m.returnErr
	}
	if m.results != nil {
		return m.results, nil
	}
	// Default: return entire content as one chunk.
	return []string{content}, nil
}

// mockEmbedder returns a single fixed vector per text.
type mockEmbedder struct {
	calledWith [][]string
	returnErr  error
}

func (m *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if m.returnErr != nil {
		return nil, m.returnErr
	}
	m.calledWith = append(m.calledWith, texts)
	vecs := make([][]float32, len(texts))
	for i := range texts {
		vecs[i] = []float32{0.1, 0.2, 0.3}
	}
	return vecs, nil
}

func (m *mockEmbedder) Dimensions() int { return 3 }
func (m *mockEmbedder) Model() string   { return "mock-embed" }

// --- Helpers ---

func buildService(repo *mockRepo, cr *mockChunkRepo, ch *mockChunker, emb Embedder) Service {
	return NewService(repo, cr, ch, emb, nil, nil, nil, nil, IngestOptions{}, nil)
}

func principal() *domain.Principal {
	return &domain.Principal{ID: "p1", Name: "Test User", Type: domain.PrincipalTypeHuman, Role: domain.InstanceRoleUser}
}

// --- Tests ---

// TestSave_Success verifies that a well-formed request creates a memory with
// a generated ID, correct fields, and calls embedding exactly once.
func TestSave_Success(t *testing.T) {
	repo := newMockRepo()
	cr := newMockChunkRepo()
	ch := &mockChunker{}
	emb := &mockEmbedder{}

	svc := buildService(repo, cr, ch, emb)

	req := SaveRequest{
		Store:     "default",
		Title:     "My Memory",
		Content:   "Some interesting content.",
		Tags:      []string{"tag1", "tag2"},
		Source:    domain.MemorySourceManual,
		Principal: principal(),
	}

	mem, err := svc.Save(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, mem)
	assert.NotEmpty(t, mem.ID)
	assert.Equal(t, "default", mem.Store)
	assert.Equal(t, "My Memory", mem.Title)
	assert.Equal(t, "Some interesting content.", mem.Content)
	assert.Equal(t, []string{"tag1", "tag2"}, mem.Tags)
	assert.Equal(t, domain.MemorySourceManual, mem.Source)
	assert.False(t, mem.CreatedAt.IsZero())
	assert.False(t, mem.UpdatedAt.IsZero())

	// Chunks must be saved and contain vectors.
	chunks := cr.chunks["default/"+mem.ID]
	require.Len(t, chunks, 1)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, chunks[0].Vector)

	// Embedder must have been called with the chunk text.
	require.Len(t, emb.calledWith, 1)
	assert.Equal(t, []string{"Some interesting content."}, emb.calledWith[0])
}

// TestSave_AutoTitle verifies that a missing title is auto-generated from content.
func TestSave_AutoTitle(t *testing.T) {
	repo := newMockRepo()
	cr := newMockChunkRepo()
	ch := &mockChunker{}
	emb := &mockEmbedder{}

	svc := buildService(repo, cr, ch, emb)

	content := "Hello world, this is a short note."
	req := SaveRequest{
		Store:   "default",
		Content: content,
	}

	mem, err := svc.Save(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, content, mem.Title)
}

// TestSave_AutoTitle_Long verifies that a long content auto-title is capped at 60 runes.
func TestSave_AutoTitle_Long(t *testing.T) {
	repo := newMockRepo()
	cr := newMockChunkRepo()
	ch := &mockChunker{}
	emb := &mockEmbedder{}

	svc := buildService(repo, cr, ch, emb)

	content := "This is a very long piece of content that definitely exceeds sixty characters in length and should be truncated."
	req := SaveRequest{
		Store:   "default",
		Content: content,
	}

	mem, err := svc.Save(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, 60, len([]rune(mem.Title)))
}

// TestSave_EmptyContent expects an INVALID_REQUEST error.
func TestSave_EmptyContent(t *testing.T) {
	svc := buildService(newMockRepo(), newMockChunkRepo(), &mockChunker{}, &mockEmbedder{})

	_, err := svc.Save(context.Background(), SaveRequest{Store: "default", Content: ""})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

// TestSave_EmptyStore expects an INVALID_REQUEST error.
func TestSave_EmptyStore(t *testing.T) {
	svc := buildService(newMockRepo(), newMockChunkRepo(), &mockChunker{}, &mockEmbedder{})

	_, err := svc.Save(context.Background(), SaveRequest{Store: "", Content: "hello"})

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeInvalidRequest, appErr.Code)
}

// TestGet_Success verifies retrieval of an existing memory.
func TestGet_Success(t *testing.T) {
	repo := newMockRepo()
	cr := newMockChunkRepo()
	svc := buildService(repo, cr, &mockChunker{}, &mockEmbedder{})

	saved, err := svc.Save(context.Background(), SaveRequest{
		Store:   "default",
		Content: "test content",
	})
	require.NoError(t, err)

	got, err := svc.Get(context.Background(), "default", saved.ID, principal())

	require.NoError(t, err)
	assert.Equal(t, saved.ID, got.ID)
}

// TestGet_NotFound expects a NOT_FOUND AppError.
func TestGet_NotFound(t *testing.T) {
	svc := buildService(newMockRepo(), newMockChunkRepo(), &mockChunker{}, &mockEmbedder{})

	_, err := svc.Get(context.Background(), "default", "nonexistent-id", principal())

	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}

// TestUpdate_TitleOnly verifies that updating only the title leaves content and
// chunks untouched (no re-chunking).
func TestUpdate_TitleOnly(t *testing.T) {
	repo := newMockRepo()
	cr := newMockChunkRepo()
	svc := buildService(repo, cr, &mockChunker{}, &mockEmbedder{})

	saved, err := svc.Save(context.Background(), SaveRequest{
		Store:   "default",
		Title:   "Old Title",
		Content: "original content",
	})
	require.NoError(t, err)

	initialSaveCalls := cr.saveCalledN
	initialDeleteCalls := cr.deleteCalledN

	newTitle := "New Title"
	updated, err := svc.Update(context.Background(), UpdateRequest{
		Store:    "default",
		MemoryID: saved.ID,
		Title:    &newTitle,
	})

	require.NoError(t, err)
	assert.Equal(t, "New Title", updated.Title)
	assert.Equal(t, "original content", updated.Content)
	// Chunks must not have been re-saved or deleted.
	assert.Equal(t, initialSaveCalls, cr.saveCalledN)
	assert.Equal(t, initialDeleteCalls, cr.deleteCalledN)
}

// TestUpdate_ContentChanged verifies that changing content triggers re-chunking.
func TestUpdate_ContentChanged(t *testing.T) {
	repo := newMockRepo()
	cr := newMockChunkRepo()
	emb := &mockEmbedder{}
	svc := buildService(repo, cr, &mockChunker{}, emb)

	saved, err := svc.Save(context.Background(), SaveRequest{
		Store:   "default",
		Content: "original content",
	})
	require.NoError(t, err)

	saveBefore := cr.saveCalledN
	deleteBefore := cr.deleteCalledN

	newContent := "completely new content"
	updated, err := svc.Update(context.Background(), UpdateRequest{
		Store:    "default",
		MemoryID: saved.ID,
		Content:  &newContent,
	})

	require.NoError(t, err)
	assert.Equal(t, "completely new content", updated.Content)
	// One additional save and one delete for re-chunking.
	assert.Equal(t, saveBefore+1, cr.saveCalledN)
	assert.Equal(t, deleteBefore+1, cr.deleteCalledN)
	// Embedder must have been called a second time.
	assert.GreaterOrEqual(t, len(emb.calledWith), 2)
}

// TestUpdate_TagsClear verifies that setting Tags to an empty slice clears all tags.
func TestUpdate_TagsClear(t *testing.T) {
	repo := newMockRepo()
	cr := newMockChunkRepo()
	svc := buildService(repo, cr, &mockChunker{}, &mockEmbedder{})

	saved, err := svc.Save(context.Background(), SaveRequest{
		Store:   "default",
		Content: "content",
		Tags:    []string{"a", "b"},
	})
	require.NoError(t, err)

	emptyTags := []string{}
	updated, err := svc.Update(context.Background(), UpdateRequest{
		Store:    "default",
		MemoryID: saved.ID,
		Tags:     &emptyTags,
	})

	require.NoError(t, err)
	assert.Empty(t, updated.Tags)
}

// TestDelete_Success verifies that both the memory and its chunks are deleted.
func TestDelete_Success(t *testing.T) {
	repo := newMockRepo()
	cr := newMockChunkRepo()
	svc := buildService(repo, cr, &mockChunker{}, &mockEmbedder{})

	saved, err := svc.Save(context.Background(), SaveRequest{
		Store:   "default",
		Content: "to be deleted",
	})
	require.NoError(t, err)

	deleteBefore := cr.deleteCalledN

	err = svc.Delete(context.Background(), "default", saved.ID, principal())

	require.NoError(t, err)
	assert.Equal(t, deleteBefore+1, cr.deleteCalledN)
	// Memory must no longer exist in the repo.
	_, getErr := svc.Get(context.Background(), "default", saved.ID, principal())
	require.Error(t, getErr)
	var appErr *domain.AppError
	require.ErrorAs(t, getErr, &appErr)
	assert.Equal(t, domain.ErrCodeNotFound, appErr.Code)
}

// TestList_Success verifies that List returns the repository result.
func TestList_Success(t *testing.T) {
	repo := newMockRepo()
	repo.listResult = []*domain.Memory{
		{ID: "m1", Store: "default", Title: "M1", Content: "c1"},
		{ID: "m2", Store: "default", Title: "M2", Content: "c2"},
	}
	repo.listTotal = 2

	svc := buildService(repo, newMockChunkRepo(), &mockChunker{}, &mockEmbedder{})

	resp, err := svc.List(context.Background(), ListRequest{
		Store:  "default",
		Limit:  10,
		Offset: 0,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Memories, 2)
	assert.Equal(t, 2, resp.Total)
}

// TestList_DefaultLimit verifies that a zero limit is replaced with the default of 20.
func TestList_DefaultLimit(t *testing.T) {
	capturingRepo := &capturingListRepo{inner: newMockRepo()}

	svc := NewService(capturingRepo, newMockChunkRepo(), &mockChunker{}, &mockEmbedder{}, nil, nil, nil, nil, IngestOptions{}, nil)

	_, err := svc.List(context.Background(), ListRequest{
		Store: "default",
		Limit: 0,
	})

	require.NoError(t, err)
	assert.Equal(t, 20, capturingRepo.lastLimit)
}

// capturingListRepo wraps mockRepo and records the limit passed to List.
type capturingListRepo struct {
	inner     *mockRepo
	lastLimit int
}

func (c *capturingListRepo) Save(ctx context.Context, m *domain.Memory) error {
	return c.inner.Save(ctx, m)
}
func (c *capturingListRepo) Get(ctx context.Context, store, id string) (*domain.Memory, error) {
	return c.inner.Get(ctx, store, id)
}
func (c *capturingListRepo) Update(ctx context.Context, m *domain.Memory) error {
	return c.inner.Update(ctx, m)
}
func (c *capturingListRepo) Delete(ctx context.Context, store, id string) error {
	return c.inner.Delete(ctx, store, id)
}
func (c *capturingListRepo) List(ctx context.Context, store string, filter Filter, limit, offset int) ([]*domain.Memory, int, error) {
	c.lastLimit = limit
	return c.inner.List(ctx, store, filter, limit, offset)
}
