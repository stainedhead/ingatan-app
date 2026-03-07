// Package templates contains the templ HTML components for the Admin WebUI.
package templates

// PrincipalRow is a row in the principal list table.
type PrincipalRow struct {
	ID, Name, Type, Role string
}

// PrincipalDetailData holds data for the principal detail page.
type PrincipalDetailData struct {
	ID, Name, Type, Role, Email string
	HasAPIKey                   bool
	CreatedAt                   string
	Memberships                 []MembershipRow
}

// MembershipRow is a row in a store membership table.
type MembershipRow struct {
	StoreName, Role string
}

// StoreRow is a row in the store list table.
type StoreRow struct {
	Name, OwnerID string
	IsPersonal    bool
	MemberCount   int
}

// StoreDetailData holds data for the store detail page.
type StoreDetailData struct {
	Name, OwnerID, Description, EmbeddingModel, CreatedAt string
	IsPersonal                                            bool
	Members                                               []MembershipRow
}
