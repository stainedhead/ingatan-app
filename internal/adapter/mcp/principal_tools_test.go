package mcp

import (
	"context"
	"testing"
	"time"

	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	principaluc "github.com/stainedhead/ingatan/internal/usecase/principal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMCPPrincipalService is a test double for principaluc.Service used by MCP tests.
type mockMCPPrincipalService struct {
	getOrCreateFn func(ctx context.Context, claims apimw.JWTClaims) (*domain.Principal, error)
	whoAmIFn      func(ctx context.Context, p *domain.Principal) (*principaluc.WhoAmIResponse, error)
	listFn        func(ctx context.Context, caller *domain.Principal) ([]*domain.Principal, error)
}

func (m *mockMCPPrincipalService) GetOrCreate(ctx context.Context, claims apimw.JWTClaims) (*domain.Principal, error) {
	return m.getOrCreateFn(ctx, claims)
}

func (m *mockMCPPrincipalService) WhoAmI(ctx context.Context, p *domain.Principal) (*principaluc.WhoAmIResponse, error) {
	return m.whoAmIFn(ctx, p)
}

func (m *mockMCPPrincipalService) List(ctx context.Context, caller *domain.Principal) ([]*domain.Principal, error) {
	return m.listFn(ctx, caller)
}

func (m *mockMCPPrincipalService) Create(_ context.Context, _ *domain.Principal, _ principaluc.CreateRequest) (*principaluc.CreateResponse, error) {
	return nil, nil
}

func (m *mockMCPPrincipalService) ReissueAPIKey(_ context.Context, _ *domain.Principal, _ string) (string, error) {
	return "", nil
}

func (m *mockMCPPrincipalService) RevokeAPIKey(_ context.Context, _ *domain.Principal, _ string) error {
	return nil
}

func (m *mockMCPPrincipalService) AuthenticateByAPIKey(_ context.Context, _ string) (*domain.Principal, error) {
	return nil, nil
}

// samplePrincipal returns a reusable test domain.Principal.
func samplePrincipal() *domain.Principal {
	return &domain.Principal{
		ID:         "user-1",
		Name:       "Test User",
		Type:       domain.PrincipalTypeHuman,
		Email:      "test@example.com",
		Role:       domain.InstanceRoleUser,
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
	}
}

func TestPrincipalHandleWhoAmI_Success(t *testing.T) {
	p := samplePrincipal()
	svc := &mockMCPPrincipalService{
		whoAmIFn: func(_ context.Context, principal *domain.Principal) (*principaluc.WhoAmIResponse, error) {
			// principalFromContext returns mcp-anonymous when no principal in ctx
			assert.Equal(t, "mcp-anonymous", principal.ID)
			return &principaluc.WhoAmIResponse{
				Principal:        p,
				StoreMemberships: []principaluc.StoreMembership{{StoreName: "my-store", Role: domain.StoreRoleOwner}},
				Capabilities:     []string{"memory:read"},
			}, nil
		},
	}

	tools := NewPrincipalTools(svc)
	req := callRequest("principal_whoami", map[string]any{})

	result, err := tools.handleWhoAmI(context.Background(), req)
	require.NoError(t, err)

	var got principaluc.WhoAmIResponse
	decodeTextResult(t, result, &got)
	require.NotNil(t, got.Principal)
	assert.Equal(t, "user-1", got.Principal.ID)
	assert.Len(t, got.StoreMemberships, 1)
	assert.Equal(t, "my-store", got.StoreMemberships[0].StoreName)
}

func TestPrincipalHandleList_Success(t *testing.T) {
	principals := []*domain.Principal{samplePrincipal()}
	svc := &mockMCPPrincipalService{
		listFn: func(_ context.Context, _ *domain.Principal) ([]*domain.Principal, error) {
			return principals, nil
		},
	}

	tools := NewPrincipalTools(svc)
	req := callRequest("principal_list", map[string]any{})

	result, err := tools.handleList(context.Background(), req)
	require.NoError(t, err)

	var got map[string]any
	decodeTextResult(t, result, &got)
	pList, ok := got["principals"].([]any)
	require.True(t, ok)
	assert.Len(t, pList, 1)
}

func TestPrincipalHandleList_Error(t *testing.T) {
	svc := &mockMCPPrincipalService{
		listFn: func(_ context.Context, _ *domain.Principal) ([]*domain.Principal, error) {
			return nil, domain.NewAppError(domain.ErrCodeForbidden, "admin role required")
		},
	}

	tools := NewPrincipalTools(svc)
	req := callRequest("principal_list", map[string]any{})

	_, err := tools.handleList(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list principals")
}
