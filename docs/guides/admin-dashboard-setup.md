# Admin Dashboard - Setup & Testing Guide

This guide shows you how to set up and test the admin dashboard in a browser.

## Quick Start

### 1. Configure Admin Users

**Option A: Using YAML Config**

```yaml
# config/manager.yaml
auth:
    admin_emails:
        - admin@example.com
        - your-email@example.com
```

**Option B: Using Environment Variable**

```bash
export ADMIN_EMAILS="admin@example.com,your-email@example.com"
```

### 2. Set Required Environment Variables

```bash
# Required: JWT secret key (minimum 32 characters)
export JWT_SECRET_KEY="your-super-secret-jwt-key-at-least-32-chars-long"

# Optional: Database path (defaults to file:./manager.db)
export DATABASE_URL="file:./manager.db"
```

### 3. Build and Run Manager

```bash
cd manager
go build ./cmd/manager
./manager
```

You should see output like:

```
INFO Starting BMC Manager Service
INFO Admin users configured admins=["admin@example.com"]
INFO Login page: http://0.0.0.0:8080/login
INFO Admin dashboard: http://0.0.0.0:8080/admin
```

## Browser Testing

### Step 1: Navigate to Login Page

Open your browser to: **http://localhost:8080/login**

You'll see a login form with:

- Email field
- Password field (not validated yet - you can enter anything)
- Sign In button

### Step 2: Login as Admin

1. Enter one of your configured admin emails (e.g., `admin@example.com`)
2. Enter any password (authentication is simplified for now)
3. Click "Sign In"

**What happens:**

- JavaScript calls the `/manager.v1.BMCManagerService/Authenticate` RPC endpoint
- Manager checks if email is in admin list
- If admin: returns JWT with `is_admin=true`
- JWT is automatically saved as `auth_token` cookie
- Browser redirects to `/admin`

### Step 3: Access Admin Dashboard

After successful login, you'll be automatically redirected to:
**http://localhost:8080/admin**

The dashboard will show:

- **Metrics Cards**: Total BMCs, Online BMCs, Gateways, Customers
- **Gateway Health Table**: Status of all gateways
- **Customer Summary**: List of customers with server counts
- **Servers Table**: All servers with advanced filtering

### Filtering Servers

Use the filter controls above the servers table:

- **Search**: Type server ID to search
- **Customer**: Filter by customer email
- **Region**: Filter by gateway region
- **Gateway**: Filter by specific gateway
- **Status**: Filter by online/offline status

## Testing Without Browser (curl)

### Get a Token

```bash
# Authenticate and get JWT token
curl -X POST http://localhost:8080/manager.v1.BMCManagerService/Authenticate \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "unused"
  }' | jq -r '.accessToken'
```

Save the token from the response.

### Access Admin API

```bash
# Set your token
TOKEN="eyJhbGc..."

# Get dashboard metrics
curl -X POST http://localhost:8080/manager.v1.AdminService/GetDashboardMetrics \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' | jq

# List all servers
curl -X POST http://localhost:8080/manager.v1.AdminService/ListAllServers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pageSize": 100}' | jq

# List all customers
curl -X POST http://localhost:8080/manager.v1.AdminService/ListAllCustomers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' | jq

# Get gateway health
curl -X POST http://localhost:8080/manager.v1.AdminService/GetGatewayHealth \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' | jq
```

## Testing with Sample Data

To see the dashboard in action, you'll need some data in the database. Here's
how to add test data:

### Using the Manager API

```bash
# Register a gateway
curl -X POST http://localhost:8080/manager.v1.BMCManagerService/RegisterGateway \
  -H "Content-Type: application/json" \
  -d '{
    "gateway_id": "gateway-test-1",
    "region": "us-west-1",
    "endpoint": "http://localhost:8081",
    "datacenter_ids": ["dc-test-1"]
  }'

# Register a server (requires authentication)
# First get a token for a customer (not admin)
TOKEN=$(curl -s -X POST http://localhost:8080/manager.v1.BMCManagerService/Authenticate \
  -H "Content-Type: application/json" \
  -d '{"email": "customer@example.com", "password": "test"}' | jq -r '.accessToken')

# Register a server
curl -X POST http://localhost:8080/manager.v1.BMCManagerService/RegisterServer \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "server_id": "server-test-1",
    "datacenter_id": "dc-test-1",
    "control_endpoints": [{
      "endpoint": "192.168.1.100:623",
      "protocol": "ipmi"
    }],
    "primary_protocol": "ipmi"
  }'
```

## Troubleshooting

### "No admin users configured" Warning

**Problem:** You see this warning when starting the manager:

```
WARN No admin users configured - admin dashboard will be inaccessible
```

**Solution:** Set admin emails in config or environment variable (see Step 1
above).

### "Forbidden: Admin privileges required" Error

**Problem:** You get a 403 error when accessing `/admin`

**Causes:**

1. You're not logged in with an admin email
2. Your email is not in the `admin_emails` list
3. Your JWT token doesn't have `is_admin=true`

**Solution:**

1. Check your config: `admin_emails` should include your email
2. Re-login at `/login` to get a new token
3. Verify your token has admin claim:
   ```bash
   # Decode JWT (install jwt-cli or use jwt.io)
   echo $TOKEN | jwt decode -
   # Look for "is_admin": true
   ```

### Empty Dashboard

**Problem:** Dashboard shows 0 for all metrics

**Cause:** No data in database yet

**Solution:** Add test data (see "Testing with Sample Data" above)

### Cookie Not Set

**Problem:** After login, you're redirected but still see "Unauthorized"

**Cause:** Cookie wasn't set properly

**Solution:**

1. Check browser console for JavaScript errors
2. Make sure you're accessing via `http://localhost:8080` (not `0.0.0.0`)
3. Clear cookies and try again
4. Check if SameSite cookie policy is blocking (shouldn't be an issue for
   localhost)

### CORS Errors

**Problem:** Browser shows CORS errors in console

**Cause:** Manager CORS middleware issue

**Solution:** The manager already has CORS enabled. If you see errors:

1. Make sure you're accessing the same origin (e.g., all `localhost:8080`)
2. Check browser console for the actual error
3. Try accessing via `http://localhost:8080` instead of `http://0.0.0.0:8080`

## Security Notes

### Development vs Production

**Current Implementation (Development):**

- Password validation not implemented (any password works)
- Simplified authentication flow
- Admin status based on email match in config

**For Production, you should:**

- Implement proper password validation
- Add database-backed customer accounts
- Use secure password hashing (bcrypt, argon2)
- Enable HTTPS/TLS
- Use secure cookie settings (Secure flag requires HTTPS)
- Implement rate limiting on login endpoint
- Add CSRF protection
- Consider OAuth2/OIDC integration

### Admin Access Control

The admin system has multiple layers of security:

1. **Configuration**: Only emails in `admin_emails` can be admins
2. **JWT Claims**: `is_admin` boolean is signed and can't be forged
3. **HTTP Handler**: `/admin` endpoint validates admin claim
4. **RPC Interceptor**: AdminService validates admin claim on every call
5. **Non-admin users**: Get 403 Forbidden with no data leakage

## Next Steps

Once you have the dashboard running with sample data:

1. **Test Filtering**: Try different filter combinations
2. **Test Sorting**: Click table headers (if implemented)
3. **Monitor Metrics**: Watch the metrics cards update
4. **Check Gateway Health**: See gateway status indicators
5. **Review Customers**: See which customers have servers

## Advanced Configuration

### Custom Port

```bash
export MANAGER_PORT=9090
```

### Enable Debug Logging

```yaml
# config/manager.yaml
log:
    level: debug
    debug: true
```

Or:

```bash
export LOG_LEVEL=debug
export DEBUG=true
```

### Database Path

```bash
export DATABASE_URL="file:/path/to/manager.db"
```

## API Documentation

### Admin RPC Endpoints

All require `Authorization: Bearer <admin-token>` header:

- `POST /manager.v1.AdminService/GetDashboardMetrics` - Get system metrics
- `POST /manager.v1.AdminService/ListAllServers` - List all servers (supports
  filtering)
- `POST /manager.v1.AdminService/ListAllCustomers` - List all customers
- `POST /manager.v1.AdminService/GetGatewayHealth` - Get gateway health status
- `POST /manager.v1.AdminService/GetRegions` - Get available regions

### Web UI Endpoints

- `GET /login`  - Login page (sets auth cookie)
- `GET /logout` - Logout page
- `GET /admin`  - Admin dashboard (requires admin cookie/token)

### Authentication Endpoint

- `POST /manager.v1.BMCManagerService/Authenticate` - Get JWT token

## Support

For issues or questions:

1. Check the logs: Manager outputs detailed logs
2. Verify configuration: Print `admin_emails` on startup
3. Test API directly: Use curl to isolate frontend issues
4. Check browser console: JavaScript errors will show there

---

**Happy Testing!** 🎉
