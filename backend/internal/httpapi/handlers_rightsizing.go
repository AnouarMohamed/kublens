package httpapi

import (
	"net/http"

	"kubelens-backend/internal/model"
	"kubelens-backend/internal/rightsizing"
)

func (s *Server) handleRightsizingOverview(w http.ResponseWriter, r *http.Request) {
	pods, _ := s.cluster.Snapshot(r.Context())
	details := make(map[string]model.PodDetail, len(pods))
	for _, pod := range pods {
		detail, err := s.cluster.PodDetail(r.Context(), pod.Namespace, pod.Name)
		if err != nil {
			continue
		}
		details[pod.Namespace+"/"+pod.Name] = detail
	}

	overview := rightsizing.BuildOverview(pods, details, collectGitOpsWorkloadInventory(r.Context(), s.cluster), s.now())
	writeJSON(w, http.StatusOK, overview)
}
