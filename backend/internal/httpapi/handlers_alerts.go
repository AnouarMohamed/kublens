package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"kubelens-backend/internal/auth"
	"kubelens-backend/internal/model"
)

type AlertController struct {
	alerts         alertDispatcher
	alertLifecycle alertLifecycleStateStore
	decodeJSONBody func(*http.Request, any) error
}

func NewAlertController(
	alerts alertDispatcher,
	alertLifecycle alertLifecycleStateStore,
	decode func(*http.Request, any) error,
) *AlertController {
	if decode == nil {
		decode = decodeJSONBody
	}
	return &AlertController{
		alerts:         alerts,
		alertLifecycle: alertLifecycle,
		decodeJSONBody: decode,
	}
}

func (ac *AlertController) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/dispatch", ac.handleAlertDispatch)
	r.Post("/test", ac.handleAlertTest)
	r.Get("/lifecycle", ac.handleListAlertLifecycle)
	r.Post("/lifecycle", ac.handleUpsertAlertLifecycle)
	return r
}

func (ac *AlertController) handleAlertDispatch(w http.ResponseWriter, r *http.Request) {
	if ac.alerts == nil || !ac.alerts.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "alert integrations are not configured")
		return
	}

	var req model.AlertDispatchRequest
	if err := ac.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Message = strings.TrimSpace(req.Message)
	if req.Title == "" || req.Message == "" {
		writeError(w, http.StatusBadRequest, "title and message are required")
		return
	}

	result := ac.alerts.Dispatch(r.Context(), req)
	status := http.StatusOK
	if !result.Success {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, result)
}

func (ac *AlertController) handleAlertTest(w http.ResponseWriter, r *http.Request) {
	if ac.alerts == nil || !ac.alerts.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "alert integrations are not configured")
		return
	}

	req := model.AlertDispatchRequest{
		Title:    "KubeLens test alert",
		Message:  "This is a test alert from KubeLens diagnostics.",
		Severity: "warning",
		Source:   "kubelens",
		Tags:     []string{"test", "diagnostics"},
	}

	result := ac.alerts.Dispatch(r.Context(), req)
	status := http.StatusOK
	if !result.Success {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, result)
}

func (ac *AlertController) handleListAlertLifecycle(w http.ResponseWriter, r *http.Request) {
	if ac.alertLifecycle == nil {
		writeJSON(w, http.StatusOK, []model.NodeAlertLifecycle{})
		return
	}
	writeJSON(w, http.StatusOK, ac.alertLifecycle.List(r.Context()))
}

func (ac *AlertController) handleUpsertAlertLifecycle(w http.ResponseWriter, r *http.Request) {
	if ac.alertLifecycle == nil {
		writeError(w, http.StatusServiceUnavailable, "alert lifecycle store is unavailable")
		return
	}

	var req model.NodeAlertLifecycleUpdateRequest
	if err := ac.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	actor := "unknown"
	if principal, ok := auth.PrincipalFromContext(r.Context()); ok {
		actor = strings.TrimSpace(principal.User)
	}

	item, err := ac.alertLifecycle.Upsert(r.Context(), req, actor)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid alert lifecycle request")
		return
	}

	writeJSON(w, http.StatusOK, item)
}
