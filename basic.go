package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/go-chi/chi/v5"
)

type Badge struct {
	Label string `json:"label"`
	Tone  string `json:"tone"`
}

type AuditKpi struct {
	Icon     string `json:"icon"`
	Label    string `json:"label"`
	Value    string `json:"value"`
	Badge    Badge  `json:"badge"`
	Sublabel string `json:"sublabel"`
}

type TimelineEvent struct {
	Time  string `json:"time"`
	Title string `json:"title"`
	Dept  string `json:"dept"`
	By    string `json:"by"`
}

type AuditStatusSegment struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type AuditOverview struct {
	Kpis            []AuditKpi           `json:"kpis"`
	Timeline        []TimelineEvent      `json:"timeline"`
	StatusBreakdown []AuditStatusSegment `json:"statusBreakdown"`
	TotalAudits     string               `json:"totalAudits"`
}
type FullProjectProfile struct {
	WorkID   string      `json:"work_id"`
	Risk     interface{} `json:"risk_profile"`
	Evidence interface{} `json:"evidence"`
	Network  interface{} `json:"network"`
}

func ProjectProfileHandler(w http.ResponseWriter, r *http.Request) {

	workID := chi.URLParam(r, "work_id")
	profile := FullProjectProfile{WorkID: workID}

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		profile.Risk = fetchFromML("https://griffingreek-ml-api.onrender.com" + workID)
	}()

	go func() {
		defer wg.Done()
		profile.Evidence = fetchFromML("https://griffingreek-ml-api.onrender.com" + workID)
	}()

	go func() {
		defer wg.Done()
		profile.Network = fetchFromML("https://griffingreek-ml-api.onrender.com" + workID)
	}()

	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

func fetchFromML(url string) interface{} {
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return nil
	}
	defer resp.Body.Close()

	var result interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}
func GetAuditOverviewHandler(w http.ResponseWriter, r *http.Request) {
	overview := AuditOverview{
		TotalAudits: "1,240",
		Kpis: []AuditKpi{
			{
				Icon:     "CheckCircle",
				Label:    "Completed",
				Value:    "850",
				Sublabel: "Last 30 days",
				Badge:    Badge{Label: "On Track", Tone: "positive"},
			},
		},
		Timeline: []TimelineEvent{
			{Time: "10:00 AM", Title: "Site Inspection", Dept: "PWD", By: "Inspector Sharma"},
		},
		StatusBreakdown: []AuditStatusSegment{
			{Label: "Completed", Value: 60},
			{Label: "Pending", Value: 30},
			{Label: "Flagged", Value: 10},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(overview)
}
func main() {
	InitDB()

	r := chi.NewRouter()

	r.Post("/api/auth/register", RegisterHandler)
	r.Post("/api/auth/login", LoginHandler)

	r.Get("/api/mp/{mp_id}/pulse-score", PulseScoreHandler)

	r.Group(func(r chi.Router) {
		r.Use(JWTMiddleware)

		r.Get("/api/projects/{work_id}/profile", ProjectProfileHandler)

		r.Post("/api/reports/generate", GenerateReportHandler)
		r.Get("/api/reports/subscriptions", ListSubscriptionsHandler)
		r.Post("/api/reports/subscriptions/{subscription_id}/trigger", TriggerSubscriptionHandler)

		r.Post("/api/alerts/evaluate", EvaluateAlertHandler)
		r.Get("/api/alerts/audit-trail", GetAuditTrailHandler)
		r.Get("/api/inspectors", GetInspectorsHandler)
		r.Get("/api/inspectors/suggest/{project_id}", SuggestInspectorsHandler)
		r.Post("/api/inspectors/dispatch", DispatchInspectorHandler)
		r.Get("/api/audit/overview", GetAuditOverviewHandler)
	})

	fmt.Println("🚀 Go Backend Gateway running on http://localhost:8080")
	http.ListenAndServe(":8080", r)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("🚀 Go Backend Gateway running on port %s\n", port)
	http.ListenAndServe(":"+port, r)
}
func PulseScoreHandler(w http.ResponseWriter, r *http.Request) {
	mpID := chi.URLParam(r, "mp_id")

	mlURL := fmt.Sprintf("https://griffingreek-ml-api.onrender.com/api/v1/pulse-score/%s", mpID)

	resp, err := http.Get(mlURL)
	if err != nil {
		http.Error(w, `{"error": "ML Service unreachable"}`, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")

	if resp.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Pulse score not available. Unknown constituency/MP, or no attributable works.",
		})
		return
	}

	if resp.StatusCode == http.StatusOK {
		var mlResponse interface{}
		if err := json.NewDecoder(resp.Body).Decode(&mlResponse); err != nil {
			http.Error(w, `{"error": "Error parsing ML data"}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mlResponse)
		return
	}

	http.Error(w, `{"error": "Unexpected response from ML service"}`, resp.StatusCode)
}

func GenerateReportHandler(w http.ResponseWriter, r *http.Request) {
	mlURL := "https://griffingreek-ml-api.onrender.com/api/v1/reports/generate"
	resp, err := http.Post(mlURL, "application/json", r.Body)
	if err != nil {
		http.Error(w, `{"error": "ML Service unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")

	io.Copy(w, resp.Body)
}

func ListSubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	mlURL := "https://griffingreek-ml-api.onrender.com/api/v1/reports/subscriptions"
	resp, err := http.Get(mlURL)
	if err != nil {
		http.Error(w, `{"error": "ML Service unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func TriggerSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	subID := chi.URLParam(r, "subscription_id")
	mlURL := fmt.Sprintf("https://griffingreek-ml-api.onrender.com/api/v1/reports/subscriptions/%s/trigger", subID)

	resp, err := http.Post(mlURL, "application/json", nil)
	if err != nil {
		http.Error(w, `{"error": "ML Service unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}
func EvaluateAlertHandler(w http.ResponseWriter, r *http.Request) {
	mlURL := "https://griffingreek-ml-api.onrender.com/api/v1/alerts/evaluate"
	resp, err := http.Post(mlURL, "application/json", r.Body)
	if err != nil {
		http.Error(w, `{"error": "ML Service unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func GetAuditTrailHandler(w http.ResponseWriter, r *http.Request) {

	mlURL := "https://griffingreek-ml-api.onrender.com/api/v1/alerts/audit-trail"
	if r.URL.RawQuery != "" {
		mlURL += "?" + r.URL.RawQuery
	}

	resp, err := http.Get(mlURL)
	if err != nil {
		http.Error(w, `{"error": "ML Service unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func GetInspectorsHandler(w http.ResponseWriter, r *http.Request) {
	mlURL := "https://griffingreek-ml-api.onrender.com/api/v1/inspectors"
	resp, err := http.Get(mlURL)
	if err != nil {
		http.Error(w, `{"error": "ML Service unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func SuggestInspectorsHandler(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	mlURL := fmt.Sprintf("https://griffingreek-ml-api.onrender.com/api/v1/inspectors/suggest/%s", projectID)

	resp, err := http.Get(mlURL)
	if err != nil {
		http.Error(w, `{"error": "ML Service unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

func DispatchInspectorHandler(w http.ResponseWriter, r *http.Request) {
	mlURL := "https://griffingreek-ml-api.onrender.com/api/v1/inspectors/dispatch"
	resp, err := http.Post(mlURL, "application/json", r.Body)
	if err != nil {
		http.Error(w, `{"error": "ML Service unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}
