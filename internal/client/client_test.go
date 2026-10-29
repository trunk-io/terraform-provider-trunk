package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/trunk-io/terraform-provider-trunk/internal/fakeapi"
)

const repoPath = "github.com/acme/widgets"

var widgets = Repo{Host: "github.com", Owner: "acme", Name: "widgets"}

func testClient(key, url string) *Client {
	c := NewClient(key, url, "test")
	c.baseRetryDelay = 0
	return c
}

func ptr[T any](v T) *T { return &v }

func TestAPIVersionForKey(t *testing.T) {
	if APIVersionForKey(fakeapi.V2Key) != V2 || APIVersionForKey(fakeapi.V1Token) != V1 {
		t.Fatal("trunk_v2_ keys use v2, anything else v1")
	}
}

func TestNewClient_baseURL(t *testing.T) {
	for _, in := range []string{"", "https://api.trunk.io", "https://api.trunk.io/", "https://api.trunk.io/v1", "https://api.trunk.io/v2/"} {
		if got := NewClient("k", in, "").host; got != "https://api.trunk.io" {
			t.Errorf("NewClient(%q).host = %q", in, got)
		}
	}
}

func TestHeaders(t *testing.T) {
	for _, tc := range []struct {
		key, header, want string
	}{
		{fakeapi.V1Token, "x-api-token", fakeapi.V1Token},
		{fakeapi.V2Key, "Authorization", "Bearer " + fakeapi.V2Key},
	} {
		var got http.Header
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Clone()
			w.WriteHeader(http.StatusNotFound)
		}))
		_, _ = testClient(tc.key, srv.URL).GetQueue(context.Background(), QueueRef{ID: "q", Repo: widgets, TargetBranch: "main"})
		srv.Close()
		if got.Get(tc.header) != tc.want {
			t.Errorf("%s = %q, want %q", tc.header, got.Get(tc.header), tc.want)
		}
		if got.Get("x-source") != "terraform" || got.Get("User-Agent") != "terraform-provider-trunk/test" {
			t.Errorf("x-source = %q, User-Agent = %q", got.Get("x-source"), got.Get("User-Agent"))
		}
		if tc.key == fakeapi.V2Key && got.Get("x-api-token") != "" {
			t.Error("v2 requests must not send the key as x-api-token")
		}
	}
}

func TestRetries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		create bool
		calls  int32
	}{
		{"5xx read is retried", http.StatusBadGateway, false, 4},
		{"429 read is retried", http.StatusTooManyRequests, false, 4},
		{"4xx is not retried", http.StatusBadRequest, false, 1},
		{"5xx create is not retried", http.StatusBadGateway, true, 1},
		{"429 create is retried", http.StatusTooManyRequests, true, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// After a failed create the client looks the queue up; count only the request under test.
				if tc.create == (r.URL.Path == "/v1/createQueue") {
					calls.Add(1)
				}
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			c := testClient(fakeapi.V1Token, srv.URL)
			if tc.create {
				_, _ = c.CreateQueue(context.Background(), CreateParams{Repo: widgets, TargetBranch: "main"})
			} else {
				_, _ = c.GetQueue(context.Background(), QueueRef{Repo: widgets, TargetBranch: "main"})
			}
			if calls.Load() != tc.calls {
				t.Errorf("calls = %d, want %d", calls.Load(), tc.calls)
			}
		})
	}
}

func TestAPIError_envelope(t *testing.T) {
	e := newAPIError(409, []byte(`{"error":{"code":"QUEUE_ALREADY_EXISTS","message":"exists"}}`))
	if e.Code != "QUEUE_ALREADY_EXISTS" || e.Message != "exists" {
		t.Errorf("v2 envelope = %+v", e)
	}
	e = newAPIError(400, []byte("bad input\n"))
	if e.Code != "" || e.Message != "bad input" {
		t.Errorf("plain body = %+v", e)
	}
}

func TestEnumConversion(t *testing.T) {
	for snake, camel := range map[string]string{
		"merge_commit": "mergeCommit", "bisection_skip_redundant_tests": "bisectionSkipRedundantTests",
		"push_new_branch_from_temp": "pushNewBranchFromTemp", "switching_modes": "switchingModes",
		"terraform_with_drift": "terraformWithDrift", "squash": "squash",
	} {
		if toCamel(snake) != camel || toSnake(camel) != snake {
			t.Errorf("%s <-> %s: got %s, %s", snake, camel, toCamel(snake), toSnake(camel))
		}
	}
}

// eachAPI runs fn against fakeapi with each credential.
func eachAPI(t *testing.T, fn func(t *testing.T, srv *fakeapi.Server, c *Client)) {
	for _, key := range []string{fakeapi.V1Token, fakeapi.V2Key} {
		name := map[string]string{fakeapi.V1Token: "v1", fakeapi.V2Key: "v2"}[key]
		t.Run(name, func(t *testing.T) {
			srv := fakeapi.New(repoPath)
			defer srv.Close()
			fn(t, srv, testClient(key, srv.URL))
		})
	}
}

func TestQueueLifecycle(t *testing.T) {
	eachAPI(t, func(t *testing.T, srv *fakeapi.Server, c *Client) {
		ctx := context.Background()
		q, err := c.CreateQueue(ctx, CreateParams{
			Repo: widgets, TargetBranch: "main", Mode: ptr(ModeParallel), State: ptr(StatePaused),
			Settings: Settings{
				Concurrency: ptr(8), BisectionConcurrency: ptr(2), EnqueueingLabel: ptr("go"),
				MergeMethod: ptr("merge_commit"), OptimizationMode: ptr("bisection_skip_redundant_tests"),
				TestBranchConstructionMode: ptr("push_new_branch_from_temp"), NotReadyTimeoutHours: ptr(3),
				RequiredStatuses: ptr([]string{"b", "a"}),
				BatchingRules:    ptr([]BatchingRule{{TargetPattern: "//x:*", Policy: "isolate"}}),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		want := &Queue{
			Repo: widgets, TargetBranch: "main", Mode: ModeParallel, State: StatePaused,
			Concurrency: 8, BisectionConcurrency: 2, EnqueueingLabel: "go", MergeMethod: "merge_commit",
			OptimizationMode: "bisection_skip_redundant_tests", TestBranchConstructionMode: "push_new_branch_from_temp",
			NotReadyTimeoutHours: 3, RequiredStatuses: []string{"a", "b"},
			BatchingRules:              []BatchingRule{{TargetPattern: "//x:*", Policy: "isolate"}},
			BatchingMaxWaitTimeMinutes: 5, BatchingMinSize: 2, CreatePrsForTestingBranches: true,
			CommentsEnabled: true, CommandsEnabled: true, StatusCheckEnabled: true, ExtensionEnabled: true,
			LabelCommandsEnabled: true, DirectMergeMode: "off",
		}
		if c.Version() == V2 {
			want.ID, want.ManagedState = q.ID, ManagedStateTerraform
		}
		if !reflect.DeepEqual(q, want) {
			t.Fatalf("created queue\n got %+v\nwant %+v", q, want)
		}

		// An empty list removes the override, on both APIs.
		q, err = c.UpdateQueue(ctx, UpdateParams{Ref: QueueRef{ID: q.ID, Repo: widgets, TargetBranch: "main"},
			State: ptr(StateRunning), Settings: Settings{RequiredStatuses: ptr([]string{})}})
		if err != nil {
			t.Fatal(err)
		}
		if len(q.RequiredStatuses) != 0 || q.State != StateRunning {
			t.Fatalf("after update: statuses %v, state %s", q.RequiredStatuses, q.State)
		}

		if _, err := c.GetQueue(ctx, QueueRef{Repo: widgets, TargetBranch: "other"}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("missing queue: %v", err)
		}
		if _, err := c.CreateQueue(ctx, CreateParams{Repo: widgets, TargetBranch: "main"}); !errors.Is(err, ErrAlreadyExists) {
			t.Fatalf("duplicate create: %v", err)
		}

		srv.Edit(repoPath, "main", func(q *fakeapi.Queue) { q.Entries = 1 })
		if err := c.DeleteQueue(ctx, QueueRef{Repo: widgets, TargetBranch: "main"}); !errors.Is(err, ErrQueueNotEmpty) {
			t.Fatalf("delete non-empty: %v", err)
		}
		srv.Edit(repoPath, "main", func(q *fakeapi.Queue) { q.Entries = 0 })
		if err := c.DeleteQueue(ctx, QueueRef{Repo: widgets, TargetBranch: "main"}); err != nil {
			t.Fatal(err)
		}
		if err := c.DeleteQueue(ctx, QueueRef{Repo: widgets, TargetBranch: "main"}); err != nil {
			t.Fatalf("deleting a missing queue should succeed: %v", err)
		}
	})
}

func TestUpdate_sendsOnlyChanges(t *testing.T) {
	eachAPI(t, func(t *testing.T, srv *fakeapi.Server, c *Client) {
		ctx := context.Background()
		q, err := c.CreateQueue(ctx, CreateParams{Repo: widgets, TargetBranch: "main"})
		if err != nil {
			t.Fatal(err)
		}
		before := len(srv.Requests())
		// The mode and state already match, so neither is sent.
		if _, err := c.UpdateQueue(ctx, UpdateParams{Ref: QueueRef{ID: q.ID, Repo: widgets, TargetBranch: "main"},
			Mode: ptr(ModeSingle), State: ptr(StateRunning), Settings: Settings{Concurrency: ptr(3)}}); err != nil {
			t.Fatal(err)
		}
		for _, r := range srv.Requests()[before:] {
			if _, ok := r.Body["mode"]; ok {
				t.Errorf("%s %s sent mode", r.Method, r.Path)
			}
			if _, ok := r.Body["state"]; ok {
				t.Errorf("%s %s sent state", r.Method, r.Path)
			}
			if r.Path == "/v2/merge-queues/"+q.ID+"/switch-modes" || r.Path == "/v2/merge-queues/"+q.ID+"/resume" {
				t.Errorf("unexpected %s", r.Path)
			}
		}
	})
}

func TestUpdate_refusedWhileSwitchingModes(t *testing.T) {
	eachAPI(t, func(t *testing.T, srv *fakeapi.Server, c *Client) {
		ctx := context.Background()
		q, err := c.CreateQueue(ctx, CreateParams{Repo: widgets, TargetBranch: "main"})
		if err != nil {
			t.Fatal(err)
		}
		ref := QueueRef{ID: q.ID, Repo: widgets, TargetBranch: "main"}
		q, err = c.UpdateQueue(ctx, UpdateParams{Ref: ref, Mode: ptr(ModeParallel)})
		if err != nil {
			t.Fatal(err)
		}
		if q.State != StateSwitchingModes || q.Mode != ModeSingle {
			t.Fatalf("mid-switch queue: state %s, mode %s", q.State, q.Mode)
		}
		if _, err := c.UpdateQueue(ctx, UpdateParams{Ref: ref, State: ptr(StatePaused)}); !errors.Is(err, ErrSwitchingModes) {
			t.Fatalf("update mid-switch: %v", err)
		}
		if got := srv.Queue(repoPath, "main").State; got != "switching_modes" {
			t.Fatalf("the refused update changed the state to %s", got)
		}
	})
}

func TestV1_removesRequiredStatusesWithFlag(t *testing.T) {
	srv := fakeapi.New(repoPath)
	defer srv.Close()
	c := testClient(fakeapi.V1Token, srv.URL)
	ctx := context.Background()
	if _, err := c.CreateQueue(ctx, CreateParams{Repo: widgets, TargetBranch: "main"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.UpdateQueue(ctx, UpdateParams{Ref: QueueRef{Repo: widgets, TargetBranch: "main"},
		Settings: Settings{RequiredStatuses: ptr([]string{})}}); err != nil {
		t.Fatal(err)
	}
	reqs := srv.Requests()
	last := reqs[len(reqs)-1].Body
	if last["deleteRequiredStatuses"] != true || last["requiredStatuses"] != nil {
		t.Errorf("v1 body = %v, want deleteRequiredStatuses and no requiredStatuses", last)
	}
}

func TestV2_notReadyTimeoutInMinutes(t *testing.T) {
	srv := fakeapi.New(repoPath)
	defer srv.Close()
	c := testClient(fakeapi.V2Key, srv.URL)
	ctx := context.Background()
	q, err := c.CreateQueue(ctx, CreateParams{Repo: widgets, TargetBranch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.UpdateQueue(ctx, UpdateParams{Ref: QueueRef{ID: q.ID}, Settings: Settings{NotReadyTimeoutHours: ptr(2)}}); err != nil {
		t.Fatal(err)
	}
	var sent any
	for _, r := range srv.Requests() {
		if r.Method == http.MethodPatch {
			sent = r.Body["notReadyTimeoutMinutes"]
		}
	}
	if sent != float64(120) {
		t.Errorf("notReadyTimeoutMinutes = %v, want 120", sent)
	}
}

func TestV2_getByIDFindsRepo(t *testing.T) {
	srv := fakeapi.New(repoPath)
	defer srv.Close()
	c := testClient(fakeapi.V2Key, srv.URL)
	ctx := context.Background()
	created, err := c.CreateQueue(ctx, CreateParams{Repo: widgets, TargetBranch: "release/1.0"})
	if err != nil {
		t.Fatal(err)
	}
	q, err := c.GetQueue(ctx, QueueRef{ID: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if q.Repo != widgets || q.TargetBranch != "release/1.0" {
		t.Errorf("repo %+v, branch %q", q.Repo, q.TargetBranch)
	}
}

func TestV2_unknownRepository(t *testing.T) {
	srv := fakeapi.New()
	defer srv.Close()
	c := testClient(fakeapi.V2Key, srv.URL)
	_, err := c.CreateQueue(context.Background(), CreateParams{Repo: widgets, TargetBranch: "main"})
	if !errors.Is(err, ErrRepositoryNotFound) || errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrRepositoryNotFound and not ErrNotFound", err)
	}
	// Reading must not report the queue as deleted either, or Terraform would forget it.
	if _, err := c.GetQueue(context.Background(), QueueRef{Repo: widgets, TargetBranch: "main"}); errors.Is(err, ErrNotFound) {
		t.Fatalf("GetQueue err = %v, must not be ErrNotFound", err)
	}
}

// A create that fails after the server committed the queue returns the queue with the error.
func TestCreate_failureAfterCommitReturnsQueue(t *testing.T) {
	eachAPI(t, func(t *testing.T, srv *fakeapi.Server, c *Client) {
		path := map[APIVersion]string{V1: "/v1/createQueue", V2: "/v2/merge-queues"}[c.Version()]
		srv.FailAfter(http.MethodPost, path, http.StatusBadGateway)
		q, err := c.CreateQueue(context.Background(), CreateParams{Repo: widgets, TargetBranch: "main"})
		if err == nil || q == nil {
			t.Fatalf("q = %v, err = %v; want both", q, err)
		}
	})
}
