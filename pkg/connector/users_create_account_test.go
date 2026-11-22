package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
)

const requestedEmail = "person@example.com"

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func checkf(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Fatalf(format, args...)
	}
}

type createAccountFake struct {
	t            *testing.T
	createStatus int
	createBody   string
	searchStatus int
	searchPages  [][]*client.User

	mu       sync.Mutex
	searches []string
	other    []string
}

func (f *createAccountFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/v4/users":
		w.WriteHeader(f.createStatus)
		_, _ = w.Write([]byte(f.createBody))
	case r.Method == http.MethodGet && r.URL.Path == "/api/v4/users":
		f.mu.Lock()
		f.searches = append(f.searches, r.URL.RawQuery)
		f.mu.Unlock()
		w.Header().Set("RateLimit-Limit", "600")
		w.Header().Set("RateLimit-Remaining", "599")
		if f.searchStatus != 0 && f.searchStatus != http.StatusOK {
			w.WriteHeader(f.searchStatus)
			_, _ = w.Write([]byte(`{"message":"lookup failed"}`))
			return
		}
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			var err error
			page, err = strconv.Atoi(p)
			must(f.t, err)
		}
		var users []*client.User
		if page <= len(f.searchPages) {
			users = f.searchPages[page-1]
		}
		if page < len(f.searchPages) {
			w.Header().Set("X-Next-Page", strconv.Itoa(page+1))
		}
		must(f.t, json.NewEncoder(w).Encode(users))
	default:
		f.mu.Lock()
		f.other = append(f.other, r.Method+" "+r.URL.Path)
		f.mu.Unlock()
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"message":"unexpected request"}`))
	}
}

func newCreateAccountFixture(t *testing.T, f *createAccountFake) *userBuilder {
	t.Helper()
	f.t = t
	server := httptest.NewServer(f)
	t.Cleanup(server.Close)
	c, err := client.New(context.Background(), "token", server.URL, "", false, false)
	must(t, err)
	checkf(t, c.IsOnPremise, "fixture must exercise the self-managed create path")
	return newUserBuilder(c)
}

func createAccountRequest(t *testing.T) (*v2.AccountInfo, *v2.LocalCredentialOptions) {
	t.Helper()
	profile, err := structpb.NewStruct(map[string]any{
		"email":    requestedEmail,
		"username": "person",
		"name":     "Person",
	})
	must(t, err)
	return &v2.AccountInfo{Profile: profile}, &v2.LocalCredentialOptions{
		Options: &v2.LocalCredentialOptions_RandomPassword_{
			RandomPassword: &v2.LocalCredentialOptions_RandomPassword{Length: 16},
		},
	}
}

func gitlabUser(id int, email, state string, bot bool) *client.User {
	return &client.User{ID: id, Email: email, Username: "user" + strconv.Itoa(id), Name: "User " + strconv.Itoa(id), State: state, Bot: bot}
}

const emailTaken = `{"message":"Email has already been taken"}`

func TestCreateAccountEmailConflictReturnsExistingUser(t *testing.T) {
	existing := gitlabUser(42, "Person@Example.com", "active", false)
	f := &createAccountFake{
		createStatus: http.StatusConflict,
		createBody:   emailTaken,
		searchPages: [][]*client.User{
			{gitlabUser(7, "other.person@example.com", "active", false)},
			{existing},
		},
	}
	builder := newCreateAccountFixture(t, f)
	info, creds := createAccountRequest(t)

	result, plaintext, annos, err := builder.CreateAccount(context.Background(), info, creds)
	must(t, err)
	got, ok := result.(*v2.CreateAccountResponse_AlreadyExistsResult)
	checkf(t, ok, "expected already-exists result, got %T", result)

	// The bound resource must be exactly what sync emits for this user.
	want, err := userResource(existing)
	must(t, err)
	checkf(t, proto.Equal(want, got.GetResource()), "want %v\ngot  %v", want, got.GetResource())
	checkf(t, got.GetResource().GetId().GetResource() == "42", "resource id = %q", got.GetResource().GetId().GetResource())

	checkf(t, len(plaintext) == 0, "no credentials may be returned for an account we did not create: %v", plaintext)
	checkf(t, len(f.other) == 0, "the existing account must not be modified: %v", f.other)
	checkf(t, len(f.searches) == 2, "searches = %v", f.searches)
	for _, q := range f.searches {
		checkf(t, strings.Contains(q, "search="+strings.ReplaceAll(requestedEmail, "@", "%40")), "query %q does not search by email", q)
	}

	var rl v2.RateLimitDescription
	outAnnos := annos
	found, err := outAnnos.Pick(&rl)
	must(t, err)
	checkf(t, found, "lookup rate-limit data must be returned as an annotation")
	checkf(t, rl.GetLimit() == 600, "rate limit = %d", rl.GetLimit())
}

func TestCreateAccountEmailConflictWithoutUnambiguousMatch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		users   []*client.User
		pages   [][]*client.User
		message string
	}{
		{name: "no exact match", users: []*client.User{gitlabUser(7, "other.person@example.com", "active", false)}, message: "no visible user"},
		{name: "email is not a primary email", users: nil, message: "no visible user"},
		{name: "blocked user", users: []*client.User{gitlabUser(42, requestedEmail, "blocked", false)}, message: "blocked"},
		{name: "deactivated user", users: []*client.User{gitlabUser(42, requestedEmail, "deactivated", false)}, message: "deactivated"},
		{name: "bot user", users: []*client.User{gitlabUser(42, requestedEmail, "active", true)}, message: "bot"},
		{
			name:    "multiple matches across pages",
			pages:   [][]*client.User{{gitlabUser(42, requestedEmail, "active", false)}, {gitlabUser(43, requestedEmail, "active", false)}},
			message: "multiple users",
		},
		{name: "multiple matches", users: []*client.User{gitlabUser(42, requestedEmail, "active", false), gitlabUser(43, strings.ToUpper(requestedEmail), "active", false)}, message: "multiple users"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &createAccountFake{
				createStatus: http.StatusConflict,
				createBody:   emailTaken,
				searchPages:  [][]*client.User{tc.users},
			}
			if tc.pages != nil {
				f.searchPages = tc.pages
			}
			builder := newCreateAccountFixture(t, f)
			info, creds := createAccountRequest(t)

			result, plaintext, _, err := builder.CreateAccount(context.Background(), info, creds)
			checkf(t, err != nil, "expected an error, got result %v", result)
			checkf(t, status.Code(err) == codes.AlreadyExists, "code = %v: %v", status.Code(err), err)
			checkf(t, strings.Contains(err.Error(), tc.message), "error %q does not mention %q", err, tc.message)
			checkf(t, result == nil, "unexpected result %v", result)
			checkf(t, len(plaintext) == 0, "unexpected plaintext")
			checkf(t, len(f.other) == 0, "unexpected requests: %v", f.other)
			checkf(t, len(f.searches) == len(f.searchPages), "searches = %v", f.searches)
		})
	}
}

func TestCreateAccountEmailConflictSameUserRepeatedAcrossPages(t *testing.T) {
	existing := gitlabUser(42, requestedEmail, "active", false)
	f := &createAccountFake{
		createStatus: http.StatusConflict,
		createBody:   emailTaken,
		searchPages:  [][]*client.User{{existing}, {existing}},
	}
	builder := newCreateAccountFixture(t, f)
	info, creds := createAccountRequest(t)

	result, plaintext, _, err := builder.CreateAccount(context.Background(), info, creds)
	must(t, err)
	got, ok := result.(*v2.CreateAccountResponse_AlreadyExistsResult)
	checkf(t, ok, "expected already-exists result, got %T", result)
	checkf(t, got.GetResource().GetId().GetResource() == "42", "resource id = %q", got.GetResource().GetId().GetResource())
	checkf(t, len(plaintext) == 0, "unexpected plaintext")
}

func TestCreateAccountEmailConflictLookupFailure(t *testing.T) {
	f := &createAccountFake{
		createStatus: http.StatusConflict,
		createBody:   emailTaken,
		searchStatus: http.StatusForbidden,
	}
	builder := newCreateAccountFixture(t, f)
	info, creds := createAccountRequest(t)

	result, plaintext, _, err := builder.CreateAccount(context.Background(), info, creds)
	checkf(t, err != nil, "expected an error, got result %v", result)
	checkf(t, status.Code(err) == codes.PermissionDenied, "code = %v: %v", status.Code(err), err)
	checkf(t, result == nil, "unexpected result %v", result)
	checkf(t, len(plaintext) == 0, "unexpected plaintext")
	checkf(t, len(f.other) == 0, "unexpected requests: %v", f.other)
}

func TestCreateAccountOtherErrorsUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		code   codes.Code
	}{
		{name: "username conflict", status: http.StatusConflict, body: `{"message":"Username has already been taken"}`, code: codes.AlreadyExists},
		{name: "conflict with validation object", status: http.StatusConflict, body: `{"message":{"email":["has already been taken"]}}`, code: codes.AlreadyExists},
		{name: "bad request", status: http.StatusBadRequest, body: `{"message":{"email":["is invalid"]}}`, code: codes.InvalidArgument},
		{name: "forbidden", status: http.StatusForbidden, body: `{"message":"403 Forbidden"}`, code: codes.PermissionDenied},
		{name: "unprocessable", status: http.StatusUnprocessableEntity, body: `{"message":"Email has already been taken"}`, code: codes.InvalidArgument},
		{name: "server error", status: http.StatusInternalServerError, body: `{"message":"500 Internal Server Error"}`, code: codes.Unavailable},
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"message":"Retry later"}`, code: codes.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &createAccountFake{createStatus: tc.status, createBody: tc.body}
			builder := newCreateAccountFixture(t, f)
			info, creds := createAccountRequest(t)

			result, plaintext, _, err := builder.CreateAccount(context.Background(), info, creds)
			checkf(t, err != nil, "expected an error, got result %v", result)
			checkf(t, status.Code(err) == tc.code, "code = %v, want %v: %v", status.Code(err), tc.code, err)
			checkf(t, strings.HasPrefix(err.Error(), "request failed: "), "error text changed: %v", err)
			if tc.status == http.StatusTooManyRequests {
				var hasRateLimit bool
				for _, d := range status.Convert(err).Details() {
					if _, ok := d.(*v2.RateLimitDescription); ok {
						hasRateLimit = true
					}
				}
				checkf(t, hasRateLimit, "rate-limit details must survive: %v", status.Convert(err).Details())
			}
			checkf(t, result == nil, "unexpected result %v", result)
			checkf(t, len(plaintext) == 0, "unexpected plaintext")
			checkf(t, len(f.searches) == 0, "only an email conflict triggers a lookup: %v", f.searches)
			checkf(t, len(f.other) == 0, "unexpected requests: %v", f.other)
		})
	}
}

func TestCreateAccountSuccessUnchanged(t *testing.T) {
	created := gitlabUser(99, requestedEmail, "active", false)
	body, err := json.Marshal(created)
	must(t, err)
	f := &createAccountFake{createStatus: http.StatusCreated, createBody: string(body)}
	builder := newCreateAccountFixture(t, f)
	info, creds := createAccountRequest(t)

	result, plaintext, _, err := builder.CreateAccount(context.Background(), info, creds)
	must(t, err)
	got, ok := result.(*v2.CreateAccountResponse_SuccessResult)
	checkf(t, ok, "expected success result, got %T", result)
	want, err := userResource(created)
	must(t, err)
	checkf(t, proto.Equal(want, got.GetResource()), "want %v\ngot  %v", want, got.GetResource())
	checkf(t, len(plaintext) == 1, "plaintext = %v", plaintext)
	checkf(t, len(plaintext[0].GetBytes()) > 0, "a newly created account still returns its generated password")
	checkf(t, len(f.searches) == 0, "searches = %v", f.searches)
	checkf(t, len(f.other) == 0, "unexpected requests: %v", f.other)
}
