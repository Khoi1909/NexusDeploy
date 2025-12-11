# Authentication Strategy - NexusDeploy

**Decision Date:** 2025-11-03  
**Owner:** DevOps Engineer + Frontend Engineer  
**Status:** DECIDED

---

## Decision: HTTP-Only Cookies

**Chosen Approach:** HTTP-only cookies set by Backend API Gateway

### Rationale

1. **Security:** Cookies with `httpOnly` flag cannot be accessed by JavaScript, protecting against XSS attacks
2. **SSR Compatible:** Next.js middleware can access cookies on server-side
3. **CSRF Protection:** Can implement CSRF tokens alongside cookies
4. **Industry Standard:** Used by major platforms (GitHub, Google, etc.)

---

## Implementation Details

### Backend (API Gateway)

After successful OAuth callback:

```go
// auth-service/handlers/oauth.go
func (h *Handler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
    // ... OAuth flow ...
    
    // Create JWT
    token := createJWT(user.ID, user.Plan)
    
    // Set HTTP-only cookie
    http.SetCookie(w, &http.Cookie{
        Name:     "nexus_token",
        Value:    token,
        Path:     "/",
        MaxAge:   86400 * 7, // 7 days
        HttpOnly: true,
        Secure:   true,  // HTTPS only in production
        SameSite: http.SameSiteLaxMode,
    })
    
    // Redirect to dashboard
    http.Redirect(w, r, "http://localhost:3000/dashboard", http.StatusFound)
}
```

### Frontend (Next.js Middleware)

```typescript
// frontend/middleware.ts
import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

export function middleware(request: NextRequest) {
  const token = request.cookies.get('nexus_token')?.value;
  const isAuthPage = request.nextUrl.pathname.startsWith('/login') || 
                     request.nextUrl.pathname.startsWith('/callback');
  
  // Redirect to login if no token and not on auth page
  if (!token && !isAuthPage) {
    return NextResponse.redirect(new URL('/login', request.url));
  }
  
  // Redirect to dashboard if has token and on auth page
  if (token && isAuthPage) {
    return NextResponse.redirect(new URL('/dashboard', request.url));
  }
  
  return NextResponse.next();
}

export const config = {
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico).*)'],
};
```

### API Calls (Frontend)

```typescript
// frontend/lib/api/client.ts
import axios from 'axios';

const apiClient = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8000',
  timeout: 10000,
  withCredentials: true, // CRITICAL: Send cookies with requests
});

// No need for Authorization header - cookies sent automatically
export default apiClient;
```

---

## Security Considerations

### CSRF Protection

**Required Implementation:**

1. Backend generates CSRF token on login
2. Token stored in non-httpOnly cookie (readable by JS)
3. Frontend reads CSRF token and sends in `X-CSRF-Token` header
4. Backend validates token matches session

```go
// Example CSRF middleware
func CSRFMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "GET" {
            sessionToken := getCSRFFromCookie(r)
            headerToken := r.Header.Get("X-CSRF-Token")
            
            if sessionToken == "" || sessionToken != headerToken {
                http.Error(w, "CSRF token invalid", http.StatusForbidden)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}
```

### Cookie Configuration

| Attribute | Value | Reason |
|:----------|:------|:-------|
| `HttpOnly` | true | Prevents JS access (XSS protection) |
| `Secure` | true (prod) | HTTPS only in production |
| `SameSite` | Lax | CSRF protection, allows GET redirects |
| `Path` | / | Cookie available for entire app |
| `MaxAge` | 604800 (7 days) | Session duration |

---

## Rejected Alternative: localStorage

**Why Not localStorage:**

1. ❌ Vulnerable to XSS attacks (any JS can read)
2. ❌ Cannot be accessed in Next.js middleware (server-side)
3. ❌ No automatic sending with requests (must manually add header)
4. ❌ Not compatible with SSR authentication

**When localStorage Is Acceptable:**
- Pure client-side SPA (no SSR)
- No sensitive data
- XSS risk is mitigated by other means

---

## Migration Path (If Changing)

If we need to switch from localStorage to cookies later:

1. Backend: Add cookie-setting endpoint
2. Frontend: One-time migration script reads localStorage, calls endpoint
3. Update all API clients to use `withCredentials: true`
4. Deploy Next.js middleware
5. Remove localStorage references

**Estimated Effort:** 4 hours

---

## Testing Checklist

- [ ] Login sets cookie (check DevTools → Application → Cookies)
- [ ] Cookie has `HttpOnly` and `Secure` flags
- [ ] Middleware redirects to `/login` when no cookie
- [ ] Middleware allows access to dashboard when cookie present
- [ ] API calls include cookie automatically (`withCredentials: true`)
- [ ] Logout clears cookie
- [ ] Cookie expires after 7 days
- [ ] CSRF protection works for POST/PUT/DELETE

---

## Documentation References

- OAuth Flow: `/docs/SRS.md` Chapter 3.1 (FR1)
- API Gateway: `/report/project-roadmaps/backend-engineer-roadmap.md`
- Frontend Auth: `/report/project-roadmaps/frontend-engineer-roadmap.md` Sprint 1

---

**Decision Owner:** DevOps Engineer  
**Approved By:** PM, Backend Lead, Frontend Lead  
**Implementation Deadline:** Before Week 1 Sprint Starts

