package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/incident"
	"kubelens-backend/internal/memory"
	"kubelens-backend/internal/model"
	"kubelens-backend/internal/postmortem"
	"kubelens-backend/internal/remediation"
	"kubelens-backend/internal/riskguard"
)

func TestIncidentStepTransitionsAndFinalVerificationRules(t *testing.T) {
	handle := newOpsTestDB(t)
	router := newOpsTestServer(
		t,
		WithIncidentStore(incident.NewStore(handle, incident.DefaultStoreLimit, nil)),
	).Router("")

	created := createIncidentForTest(t, router)
	steps := created.Runbook
	if len(steps) == 0 {
		t.Fatal("expected incident runbook steps")
	}

	finalStep := steps[len(steps)-1]
	if !finalStep.Mandatory {
		t.Fatalf("expected final step to be mandatory: %#v", finalStep)
	}

	patchFinal := httptest.NewRequest(http.MethodPatch, "/api/incidents/"+created.ID+"/steps/"+finalStep.ID, strings.NewReader(`{"status":"skipped"}`))
	patchFinal.Header.Set("Authorization", "Bearer operator-token")
	patchFinal.Header.Set("Content-Type", "application/json")
	patchFinalResp := httptest.NewRecorder()
	router.ServeHTTP(patchFinalResp, patchFinal)
	if patchFinalResp.Code != http.StatusBadRequest {
		t.Fatalf("final skip status = %d, want 400", patchFinalResp.Code)
	}
	assertErrorContains(t, patchFinalResp, "final verification step cannot be skipped")

	target := steps[0]
	invalidTransition := httptest.NewRequest(http.MethodPatch, "/api/incidents/"+created.ID+"/steps/"+target.ID, strings.NewReader(`{"status":"done"}`))
	invalidTransition.Header.Set("Authorization", "Bearer operator-token")
	invalidTransition.Header.Set("Content-Type", "application/json")
	invalidTransitionResp := httptest.NewRecorder()
	router.ServeHTTP(invalidTransitionResp, invalidTransition)
	if invalidTransitionResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid transition status = %d, want 400", invalidTransitionResp.Code)
	}
	assertErrorContains(t, invalidTransitionResp, "invalid status transition:")

	stepToProgress := httptest.NewRequest(http.MethodPatch, "/api/incidents/"+created.ID+"/steps/"+target.ID, strings.NewReader(`{"status":"in_progress"}`))
	stepToProgress.Header.Set("Authorization", "Bearer operator-token")
	stepToProgress.Header.Set("Content-Type", "application/json")
	stepToProgressResp := httptest.NewRecorder()
	router.ServeHTTP(stepToProgressResp, stepToProgress)
	if stepToProgressResp.Code != http.StatusOK {
		t.Fatalf("pending->in_progress status = %d, want 200", stepToProgressResp.Code)
	}

	stepToDone := httptest.NewRequest(http.MethodPatch, "/api/incidents/"+created.ID+"/steps/"+target.ID, strings.NewReader(`{"status":"done"}`))
	stepToDone.Header.Set("Authorization", "Bearer operator-token")
	stepToDone.Header.Set("Content-Type", "application/json")
	stepToDoneResp := httptest.NewRecorder()
	router.ServeHTTP(stepToDoneResp, stepToDone)
	if stepToDoneResp.Code != http.StatusOK {
		t.Fatalf("in_progress->done status = %d, want 200", stepToDoneResp.Code)
	}
}

func TestRemediationFourEyesEnforcement(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		wantStatus int
	}{
		{name: "prod blocks same approver executor", mode: "prod", wantStatus: http.StatusForbidden},
		{name: "demo allows same approver executor", mode: "demo", wantStatus: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handle := newOpsTestDB(t)
			remStore := remediation.NewStore(handle, remediation.DefaultStoreLimit, nil)
			seed := remStore.SaveProposals([]model.RemediationProposal{
				{
					Kind:         model.RemediationKindRestartPod,
					Resource:     "payment-gateway-1",
					Namespace:    "production",
					Reason:       "critical pod restart loop detected",
					RiskLevel:    "low",
					DryRunResult: "Pod payment-gateway-1 in namespace production would be restarted.",
				},
			})
			if len(seed) == 0 {
				t.Fatal("failed to seed remediation proposals")
			}

			router := newOpsTestServer(
				t,
				WithRemediationStore(remStore),
				WithRuntimeStatus(model.RuntimeStatus{Mode: tc.mode}),
				WithWriteActionsEnabled(true),
			).Router("")
			target := seed[0]

			approveReq := httptest.NewRequest(http.MethodPost, "/api/remediation/"+target.ID+"/approve", strings.NewReader(`{}`))
			approveReq.Header.Set("Authorization", "Bearer operator-token")
			approveReq.Header.Set("Content-Type", "application/json")
			approveResp := httptest.NewRecorder()
			router.ServeHTTP(approveResp, approveReq)
			if approveResp.Code != http.StatusOK {
				t.Fatalf("approve status = %d, want 200", approveResp.Code)
			}

			executeReq := httptest.NewRequest(http.MethodPost, "/api/remediation/"+target.ID+"/execute", strings.NewReader(`{}`))
			executeReq.Header.Set("Authorization", "Bearer operator-token")
			executeReq.Header.Set("Content-Type", "application/json")
			executeResp := httptest.NewRecorder()
			router.ServeHTTP(executeResp, executeReq)
			if executeResp.Code != tc.wantStatus {
				t.Fatalf("execute status = %d, want %d", executeResp.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusForbidden {
				assertErrorContains(t, executeResp, "four-eyes enforcement: the approver and executor must be different users")
			}
		})
	}
}

func TestProposedRemediationLinksToOpenIncident(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handle := newOpsTestDB(t)
	router := newServer(
		notReadyClusterReader{testClusterReader{}},
		nil,
		logger,
		WithWriteActionsEnabled(true),
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens: []AuthToken{
				{Token: "viewer-token", User: "viewer", Role: "viewer"},
				{Token: "operator-token", User: "operator", Role: "operator"},
				{Token: "admin-token", User: "admin", Role: "admin"},
			},
		}),
		WithSQLiteDB(handle),
		WithIncidentStore(incident.NewStore(handle, incident.DefaultStoreLimit, nil)),
		WithRemediationStore(remediation.NewStore(handle, remediation.DefaultStoreLimit, nil)),
	).Router("")

	created := createIncidentForTest(t, router)
	proposeReq := httptest.NewRequest(http.MethodPost, "/api/remediation/propose", strings.NewReader(`{}`))
	proposeReq.Header.Set("Authorization", "Bearer viewer-token")
	proposeReq.Header.Set("Content-Type", "application/json")
	proposeResp := httptest.NewRecorder()
	router.ServeHTTP(proposeResp, proposeReq)
	if proposeResp.Code != http.StatusOK {
		t.Fatalf("propose status = %d, want 200", proposeResp.Code)
	}

	var proposals []model.RemediationProposal
	if err := json.NewDecoder(proposeResp.Body).Decode(&proposals); err != nil {
		t.Fatalf("decode proposals: %v", err)
	}
	if len(proposals) == 0 {
		t.Fatal("expected proposals to be generated")
	}

	linked := false
	for _, proposal := range proposals {
		if proposal.IncidentID == created.ID {
			linked = true
			break
		}
	}
	if !linked {
		t.Fatalf("expected at least one proposal linked to incident %s; proposals=%+v", created.ID, proposals)
	}
}

func TestApplyResourceYAMLRiskGuardForceFlow(t *testing.T) {
	router := newOpsTestServer(
		t,
		WithRiskAnalyzer(riskguard.NewAnalyzer()),
		WithWriteActionsEnabled(true),
	).Router("")

	manifest := `{"yaml":"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: payment-gateway\n  namespace: production\nspec:\n  replicas: 1\n  template:\n    spec:\n      containers:\n      - name: app\n        image: ghcr.io/example/payment-gateway:latest\n"}`
	blockReq := httptest.NewRequest(http.MethodPut, "/api/resources/deployments/production/payment-gateway/yaml", strings.NewReader(manifest))
	blockReq.Header.Set("Authorization", "Bearer operator-token")
	blockReq.Header.Set("Content-Type", "application/json")
	blockResp := httptest.NewRecorder()
	router.ServeHTTP(blockResp, blockReq)
	if blockResp.Code != http.StatusAccepted {
		t.Fatalf("risk block status = %d, want 202", blockResp.Code)
	}

	var blocked model.ResourceApplyRiskResponse
	if err := json.NewDecoder(blockResp.Body).Decode(&blocked); err != nil {
		t.Fatalf("decode blocked response: %v", err)
	}
	if !blocked.RequiresForce || blocked.Report.Score < 50 {
		t.Fatalf("unexpected blocked payload: %#v", blocked)
	}

	forceReq := httptest.NewRequest(http.MethodPut, "/api/resources/deployments/production/payment-gateway/yaml?force=true", strings.NewReader(manifest))
	forceReq.Header.Set("Authorization", "Bearer operator-token")
	forceReq.Header.Set("Content-Type", "application/json")
	forceResp := httptest.NewRecorder()
	router.ServeHTTP(forceResp, forceReq)
	if forceResp.Code != http.StatusOK {
		t.Fatalf("force apply status = %d, want 200", forceResp.Code)
	}

	auditReq := httptest.NewRequest(http.MethodGet, "/api/audit?limit=50", nil)
	auditReq.Header.Set("Authorization", "Bearer admin-token")
	auditResp := httptest.NewRecorder()
	router.ServeHTTP(auditResp, auditReq)
	if auditResp.Code != http.StatusOK {
		t.Fatalf("audit status = %d, want 200", auditResp.Code)
	}

	var payload model.AuditLogResponse
	if err := json.NewDecoder(auditResp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode audit payload: %v", err)
	}
	foundOverride := false
	for _, item := range payload.Items {
		if strings.Contains(item.Action, "resource.apply.force_override riskScore=") {
			foundOverride = true
			break
		}
	}
	if !foundOverride {
		t.Fatal("expected force override audit entry")
	}
}

func TestPostmortemGenerationFlow(t *testing.T) {
	handle := newOpsTestDB(t)
	router := newOpsTestServer(
		t,
		WithSQLiteDB(handle),
		WithIncidentStore(incident.NewStore(handle, incident.DefaultStoreLimit, nil)),
		WithPostmortemStore(postmortem.NewStore(handle, postmortem.DefaultStoreLimit, nil)),
	).Router("")

	created := createIncidentForTest(t, router)

	preResolveReq := httptest.NewRequest(http.MethodPost, "/api/incidents/"+created.ID+"/postmortem", strings.NewReader(`{}`))
	preResolveReq.Header.Set("Authorization", "Bearer operator-token")
	preResolveReq.Header.Set("Content-Type", "application/json")
	preResolveResp := httptest.NewRecorder()
	router.ServeHTTP(preResolveResp, preResolveReq)
	if preResolveResp.Code != http.StatusBadRequest {
		t.Fatalf("pre-resolve postmortem status = %d, want 400", preResolveResp.Code)
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/api/incidents/"+created.ID+"/resolve", strings.NewReader(`{}`))
	resolveReq.Header.Set("Authorization", "Bearer operator-token")
	resolveReq.Header.Set("Content-Type", "application/json")
	completeIncidentRunbookForTest(t, router, created.ID)
	resolveResp := httptest.NewRecorder()
	router.ServeHTTP(resolveResp, resolveReq)
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200", resolveResp.Code)
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/api/incidents/"+created.ID+"/postmortem", strings.NewReader(`{}`))
	firstReq.Header.Set("Authorization", "Bearer operator-token")
	firstReq.Header.Set("Content-Type", "application/json")
	firstResp := httptest.NewRecorder()
	router.ServeHTTP(firstResp, firstReq)
	if firstResp.Code != http.StatusCreated {
		t.Fatalf("first postmortem status = %d, want 201", firstResp.Code)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/incidents/"+created.ID+"/postmortem", strings.NewReader(`{}`))
	secondReq.Header.Set("Authorization", "Bearer operator-token")
	secondReq.Header.Set("Content-Type", "application/json")
	secondResp := httptest.NewRecorder()
	router.ServeHTTP(secondResp, secondReq)
	if secondResp.Code != http.StatusConflict {
		t.Fatalf("second postmortem status = %d, want 409", secondResp.Code)
	}
	assertErrorContains(t, secondResp, "postmortem already exists for incident:")
}

func TestMemoryEndpointsCRUD(t *testing.T) {
	tempDir := t.TempDir()
	store := memory.New(filepath.Join(tempDir, "memory.json"), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	router := newOpsTestServer(t, WithMemoryStore(store)).Router("")

	createRunbookReq := httptest.NewRequest(http.MethodPost, "/api/memory/runbooks", strings.NewReader(`{"title":"OOM restart","tags":["oom","payments"],"description":"Handle pod OOM","steps":["Inspect events","Restart pod"]}`))
	createRunbookReq.Header.Set("Authorization", "Bearer operator-token")
	createRunbookReq.Header.Set("Content-Type", "application/json")
	createRunbookResp := httptest.NewRecorder()
	router.ServeHTTP(createRunbookResp, createRunbookReq)
	if createRunbookResp.Code != http.StatusCreated {
		t.Fatalf("create runbook status = %d, want 201", createRunbookResp.Code)
	}

	var created model.MemoryRunbook
	if err := json.NewDecoder(createRunbookResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode runbook: %v", err)
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/api/memory/runbooks?q=oom", nil)
	searchReq.Header.Set("Authorization", "Bearer viewer-token")
	searchResp := httptest.NewRecorder()
	router.ServeHTTP(searchResp, searchReq)
	if searchResp.Code != http.StatusOK {
		t.Fatalf("search runbook status = %d, want 200", searchResp.Code)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/memory/runbooks/"+created.ID, strings.NewReader(`{"title":"OOM restart v2","tags":["oom"],"description":"Updated flow","steps":["Inspect","Restart","Verify"]}`))
	updateReq.Header.Set("Authorization", "Bearer operator-token")
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	router.ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update runbook status = %d, want 200", updateResp.Code)
	}

	recordFixReq := httptest.NewRequest(http.MethodPost, "/api/memory/fixes", strings.NewReader(`{"incidentId":"inc-1","proposalId":"rem-1","title":"Rollback fixed error budget burn","description":"Rollback restored service","resource":"production/payment-gateway","kind":"rollback_deployment"}`))
	recordFixReq.Header.Set("Authorization", "Bearer operator-token")
	recordFixReq.Header.Set("Content-Type", "application/json")
	recordFixResp := httptest.NewRecorder()
	router.ServeHTTP(recordFixResp, recordFixReq)
	if recordFixResp.Code != http.StatusCreated {
		t.Fatalf("record fix status = %d, want 201", recordFixResp.Code)
	}

	listFixesReq := httptest.NewRequest(http.MethodGet, "/api/memory/fixes", nil)
	listFixesReq.Header.Set("Authorization", "Bearer viewer-token")
	listFixesResp := httptest.NewRecorder()
	router.ServeHTTP(listFixesResp, listFixesReq)
	if listFixesResp.Code != http.StatusOK {
		t.Fatalf("list fixes status = %d, want 200", listFixesResp.Code)
	}

	var fixes []model.MemoryFixPattern
	if err := json.NewDecoder(listFixesResp.Body).Decode(&fixes); err != nil {
		t.Fatalf("decode fixes: %v", err)
	}
	if len(fixes) != 1 {
		t.Fatalf("fixes length = %d, want 1", len(fixes))
	}

	filterFixesReq := httptest.NewRequest(http.MethodGet, "/api/memory/fixes?q=rollback", nil)
	filterFixesReq.Header.Set("Authorization", "Bearer viewer-token")
	filterFixesResp := httptest.NewRecorder()
	router.ServeHTTP(filterFixesResp, filterFixesReq)
	if filterFixesResp.Code != http.StatusOK {
		t.Fatalf("filtered fixes status = %d, want 200", filterFixesResp.Code)
	}

	var filtered []model.MemoryFixPattern
	if err := json.NewDecoder(filterFixesResp.Body).Decode(&filtered); err != nil {
		t.Fatalf("decode filtered fixes: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("filtered fixes length = %d, want 1", len(filtered))
	}
}

func newOpsTestServer(t *testing.T, opts ...Option) *Server {
	t.Helper()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handle := newOpsTestDB(t)
	base := []Option{
		WithSQLiteDB(handle),
		WithWriteActionsEnabled(true),
		WithAuth(AuthConfig{
			Enabled: true,
			Tokens: []AuthToken{
				{Token: "viewer-token", User: "viewer", Role: "viewer"},
				{Token: "operator-token", User: "operator", Role: "operator"},
				{Token: "admin-token", User: "admin", Role: "admin"},
			},
		}),
	}
	base = append(base, opts...)
	return newServer(testClusterReader{}, nil, logger, base...)
}

func newOpsTestDB(t *testing.T) *sql.DB {
	t.Helper()

	handle, err := storesql.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	t.Cleanup(func() {
		_ = handle.Close()
	})

	return handle
}

func createIncidentForTest(t *testing.T, router http.Handler) model.Incident {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/incidents", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer viewer-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create incident status = %d, want 201", resp.Code)
	}

	var created model.Incident
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created incident: %v", err)
	}
	return created
}

func assertErrorContains(t *testing.T, rr *httptest.ResponseRecorder, contains string) {
	t.Helper()
	var payload map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if !strings.Contains(payload["error"], contains) {
		t.Fatalf("error %q does not contain %q", payload["error"], contains)
	}
}

func completeIncidentRunbookForTest(t *testing.T, router http.Handler, incidentID string) {
	t.Helper()

	getReq := httptest.NewRequest(http.MethodGet, "/api/incidents/"+incidentID, nil)
	getReq.Header.Set("Authorization", "Bearer viewer-token")
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get incident status = %d, want 200", getResp.Code)
	}

	var incident model.Incident
	if err := json.NewDecoder(getResp.Body).Decode(&incident); err != nil {
		t.Fatalf("decode incident detail: %v", err)
	}

	for _, step := range incident.Runbook {
		if step.Status == model.RunbookStepStatusDone || step.Status == model.RunbookStepStatusSkipped {
			continue
		}

		if step.Status == model.RunbookStepStatusPending {
			progressReq := httptest.NewRequest(
				http.MethodPatch,
				"/api/incidents/"+incidentID+"/steps/"+step.ID,
				strings.NewReader(`{"status":"in_progress"}`),
			)
			progressReq.Header.Set("Authorization", "Bearer operator-token")
			progressReq.Header.Set("Content-Type", "application/json")
			progressResp := httptest.NewRecorder()
			router.ServeHTTP(progressResp, progressReq)
			if progressResp.Code != http.StatusOK {
				t.Fatalf("step %s to in_progress status = %d, want 200", step.ID, progressResp.Code)
			}
		}

		doneReq := httptest.NewRequest(
			http.MethodPatch,
			"/api/incidents/"+incidentID+"/steps/"+step.ID,
			strings.NewReader(`{"status":"done"}`),
		)
		doneReq.Header.Set("Authorization", "Bearer operator-token")
		doneReq.Header.Set("Content-Type", "application/json")
		doneResp := httptest.NewRecorder()
		router.ServeHTTP(doneResp, doneReq)
		if doneResp.Code != http.StatusOK {
			t.Fatalf("step %s to done status = %d, want 200", step.ID, doneResp.Code)
		}
	}
}

type notReadyClusterReader struct {
	testClusterReader
}

func (notReadyClusterReader) Snapshot(context.Context) ([]model.PodSummary, []model.NodeSummary) {
	return []model.PodSummary{
			{Name: "payment-gateway-1", Namespace: "production", Status: model.PodStatusRunning, Restarts: 0},
		}, []model.NodeSummary{
			{Name: "node-1", Status: model.NodeStatusNotReady},
		}
}
