package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"kubelens-backend/internal/model"
	"kubelens-backend/internal/rightsizing"
)

type RightsizingController struct {
	cluster ClusterReader
	now     func() time.Time
}

func NewRightsizingController(cluster ClusterReader, now func() time.Time) *RightsizingController {
	if now == nil {
		now = time.Now
	}
	return &RightsizingController{
		cluster: cluster,
		now:     now,
	}
}

func (rc *RightsizingController) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", rc.handleRightsizingOverview)
	return r
}

func (rc *RightsizingController) handleRightsizingOverview(w http.ResponseWriter, r *http.Request) {
	pods, _ := rc.cluster.Snapshot(r.Context())
	details := make(map[string]model.PodDetail, len(pods))
	for _, pod := range pods {
		detail, err := rc.cluster.PodDetail(r.Context(), pod.Namespace, pod.Name)
		if err != nil {
			continue
		}
		details[pod.Namespace+"/"+pod.Name] = detail
	}

	overview := rightsizing.BuildOverview(pods, details, collectGitOpsWorkloadInventory(r.Context(), rc.cluster), rc.now())
	writeJSON(w, http.StatusOK, overview)
}
