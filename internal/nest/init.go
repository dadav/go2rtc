package nest

import (
	"net/http"

	"github.com/dadav/go2rtc/internal/api"
	"github.com/dadav/go2rtc/internal/streams"
	"github.com/dadav/go2rtc/pkg/core"
	"github.com/dadav/go2rtc/pkg/nest"
)

func Init() {
	streams.HandleFunc("nest", streamNest)

	api.HandleFunc("api/nest", apiNest)
}

func streamNest(url string) (core.Producer, error) {
	client, err := nest.NewClient(url)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func apiNest(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	cliendID := query.Get("client_id")
	cliendSecret := query.Get("client_secret")
	refreshToken := query.Get("refresh_token")
	projectID := query.Get("project_id")

	nestAPI, err := nest.NewAPI(cliendID, cliendSecret, refreshToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	devices, err := nestAPI.GetDevices(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []*api.Source

	for name, deviceID := range devices {
		query.Set("device_id", deviceID)

		items = append(items, &api.Source{
			Name: name, URL: "nest:?" + query.Encode(),
		})
	}

	api.ResponseSources(w, items)
}
