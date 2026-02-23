package middleware

import (
	"context"
	"net/http"
)

type ctxKey string

const tenantKey ctxKey = "tenant"

func Tenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := r.Header.Get("X-Tenant-ID")
		if tid == "" {
			http.Error(w, "Tenant required", http.StatusBadRequest)
			return
		}
		ctx := context.WithValue(r.Context(), tenantKey, tid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetTenantID(ctx context.Context) string {
	if v := ctx.Value(tenantKey); v != nil {
		return v.(string)
	}
	return ""
}

//Personal.AI order the ending
