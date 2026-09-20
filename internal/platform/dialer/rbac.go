package dialer

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Role defines the authorization level of an authenticated subject.
type Role string

const (
	RoleAdmin    Role = "ADMIN"
	RoleOperator Role = "OPERATOR"
	RoleViewer   Role = "VIEWER"
)

// Permission defines operational capability.
type Permission string

const (
	PermRead    Permission = "READ"    // View metrics, configs, read-only status
	PermOperate Permission = "OPERATE" // Start, pause, resume campaigns, claim leads, trigger calls
	PermAdmin   Permission = "ADMIN"   // Mutate tenant configs, credentials, delete resources, admin endpoints
)

var (
	ErrUnauthorized         = errors.New("unauthorized: missing or invalid credentials")
	ErrPermissionDenied     = errors.New("forbidden: insufficient role permissions")
	ErrCrossTenantForbidden = errors.New("forbidden: cross-tenant access denied")
)

// TenantIdentity encapsulates the authenticated tenant identity and its granted role.
type TenantIdentity struct {
	TenantID string `json:"tenant_id"`
	Role     Role   `json:"role"`
	APIKey   string `json:"-"` // Never serialized
}

// HasPermission checks if the role satisfies the required permission level.
func (r Role) HasPermission(perm Permission) bool {
	switch perm {
	case PermRead:
		return r == RoleAdmin || r == RoleOperator || r == RoleViewer
	case PermOperate:
		return r == RoleAdmin || r == RoleOperator
	case PermAdmin:
		return r == RoleAdmin
	default:
		return false
	}
}

// Authorize validates that the caller is authorized to act on the target tenant with the required permission.
func Authorize(identity *TenantIdentity, targetTenant string, requiredPerm Permission) error {
	if identity == nil {
		return ErrUnauthorized
	}

	// 1. Permission check
	if !identity.Role.HasPermission(requiredPerm) {
		return fmt.Errorf("%w: role %s cannot perform %s", ErrPermissionDenied, identity.Role, requiredPerm)
	}

	// 2. Multi-tenant isolation: ADMIN with global scope can manage any tenant; others can only access own tenant
	if identity.Role != RoleAdmin && identity.TenantID != targetTenant {
		return fmt.Errorf("%w: caller tenant %s cannot access target %s", ErrCrossTenantForbidden, identity.TenantID, targetTenant)
	}

	return nil
}

// AuthenticateRequest extracts and verifies identity from request headers / query params.
// Enforces that client-supplied X-Tenant-ID cannot override identity unless the caller is ADMIN.
func AuthenticateRequest(req *http.Request) (*TenantIdentity, error) {
	apiKey := req.Header.Get("X-API-Key")
	if apiKey == "" {
		apiKey = req.URL.Query().Get("apiKey")
	}
	if apiKey == "" {
		return nil, ErrUnauthorized
	}

	// 1. Master Key -> ADMIN Role
	masterKey := os.Getenv("WACALLS_API_KEY")
	if masterKey != "" && apiKey == masterKey {
		tenantID := req.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "admin-tenant"
		}
		return &TenantIdentity{
			TenantID: tenantID,
			Role:     RoleAdmin,
			APIKey:   apiKey,
		}, nil
	}

	// 2. Tenant Scoped Key format: "tenant-<tenantID>-<role>" or "tenant-<tenantID>"
	if strings.HasPrefix(apiKey, "tenant-") {
		parts := strings.Split(strings.TrimPrefix(apiKey, "tenant-"), "-")
		if len(parts) == 0 || parts[0] == "" {
			return nil, ErrUnauthorized
		}

		tenantID := parts[0]
		role := RoleOperator // Default role for tenant keys
		if len(parts) > 1 {
			switch strings.ToUpper(parts[1]) {
			case "ADMIN":
				role = RoleAdmin
			case "VIEWER":
				role = RoleViewer
			case "OPERATOR":
				role = RoleOperator
			}
		}

		// Security: Prevent cross-tenant forgery via header injection
		clientHeaderTenant := req.Header.Get("X-Tenant-ID")
		if clientHeaderTenant != "" && clientHeaderTenant != tenantID {
			return nil, ErrCrossTenantForbidden
		}

		return &TenantIdentity{
			TenantID: tenantID,
			Role:     role,
			APIKey:   apiKey,
		}, nil
	}

	// 3. Widget Key (ReadOnly / Viewer role)
	widgetKey := os.Getenv("WACALLS_WIDGET_KEY")
	if widgetKey != "" && apiKey == widgetKey {
		return &TenantIdentity{
			TenantID: "widget-tenant",
			Role:     RoleViewer,
			APIKey:   apiKey,
		}, nil
	}

	return nil, ErrUnauthorized
}
