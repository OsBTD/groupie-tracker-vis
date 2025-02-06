package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Artists represents the artist data structure
type Artists struct {
	Image          string   `json:"image"`
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	Members        []string `json:"members"`
	CreationDate   int      `json:"creationDate"`
	FirstAlbum     string   `json:"firstAlbum"`
	RelationsURL   string   `json:"relations"`
	DatesLocations Relations
	Likes          int
}

// Relations represents the concert dates and locations data
type Relations struct {
	ID             int                 `json:"id"`
	DatesLocations map[string][]string `json:"datesLocations"`
}

// RelationsResponse represents the API response structure
type RelationsResponse struct {
	Index []Relations `json:"index"`
}

// ErrorPage represents the data structure for error information
type ErrorPage struct {
	Code    int
	Message string
	Is405   bool
	Is404   bool
	Is500   bool
	Is403   bool
	Is400   bool
}

// fetchData makes an HTTP GET request and decodes the JSON response
func fetchData(url string, target interface{}) error {
	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error making GET request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 response code: %d", response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

// handleError handles error responses consistently across handlers
func handleError(w http.ResponseWriter, tmpl *template.Template, code int, message string) {
	errorPage := ErrorPage{
		Code:    code,
		Message: message,
		Is405:   code == http.StatusMethodNotAllowed,
		Is404:   code == http.StatusNotFound,
		Is500:   code == http.StatusInternalServerError,
		Is403:   code == http.StatusForbidden,
		Is400:   code == http.StatusBadRequest,
	}
	w.WriteHeader(code)
	if err := tmpl.Execute(w, errorPage); err != nil {
		log.Printf("Error executing error template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

var likes = make(map[int]int) // Map to track likes

func main() {
	// Parse templates
	templates := make(map[string]*template.Template)
	templateFiles := map[string]string{
		"index":  "templates/index.html",
		"error":  "templates/error.html",
		"about":  "templates/about.html",
		"readme": "templates/readme.html",
	}

	for name, file := range templateFiles {
		tmpl, err := template.ParseFiles(file)
		if err != nil {
			log.Fatalf("Error parsing template %s: %v", name, err)
		}
		templates[name] = tmpl
	}

	// Fetch and prepare data
	var artists []Artists
	var relationsResponse RelationsResponse

	// Fetch relations data
	if err := fetchData("https://groupietrackers.herokuapp.com/api/relation", &relationsResponse); err != nil {
		log.Printf("Error fetching relations: %v", err)
	}

	// Fetch artists data
	if err := fetchData("https://groupietrackers.herokuapp.com/api/artists", &artists); err != nil {
		log.Printf("Error fetching artists: %v", err)
	}

	// Map relations to artists
	relationsMap := make(map[int]Relations)
	for _, relation := range relationsResponse.Index {
		relationsMap[relation.ID] = relation
	}
	for i := range artists {
		relation, found := relationsMap[artists[i].ID]
		if found {
			artists[i].DatesLocations = relation
		}
	}

	// Define route handlers
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			handleError(w, templates["error"], http.StatusNotFound, "Page not found")
			return
		}

		if r.Method != http.MethodGet {
			handleError(w, templates["error"], http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		for i := range artists {
			if count, exists := likes[artists[i].ID]; exists {
				artists[i].Likes = count
			}
		}

		if err := templates["index"].Execute(w, artists); err != nil {
			log.Printf("Error executing index template: %v", err)
			handleError(w, templates["error"], http.StatusInternalServerError, "Internal server error")
		}
	})

	http.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/about" {
			handleError(w, templates["error"], http.StatusNotFound, "Page not found")
			return
		}

		if r.Method != http.MethodGet {
			handleError(w, templates["error"], http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		if err := templates["about"].Execute(w, nil); err != nil {
			log.Printf("Error executing about template: %v", err)
			handleError(w, templates["error"], http.StatusInternalServerError, "Internal server error")
		}
	})

	http.HandleFunc("/readme", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readme" {
			handleError(w, templates["error"], http.StatusNotFound, "Page not found")
			return
		}

		if r.Method != http.MethodGet {
			handleError(w, templates["error"], http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		if err := templates["readme"].Execute(w, nil); err != nil {
			log.Printf("Error executing readme template: %v", err)
			handleError(w, templates["error"], http.StatusInternalServerError, "Internal server error")
		}
	})
	http.HandleFunc("/like/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			handleError(w, templates["error"], http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		artistIDStr := strings.TrimPrefix(r.URL.Path, "/like/")
		artistID, err := strconv.Atoi(artistIDStr)
		if err != nil {
			handleError(w, templates["error"], http.StatusBadRequest, "Invalid artist ID")
			return
		}
		for i := range artists {
			if artists[i].ID == artistID {
				artists[i].Likes = likes[artistID]
				break
			}
		}

		likes[artistID]++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(likes[artistID])
	})

	// Serve static files
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		subPath := strings.TrimPrefix(r.URL.Path, "/static/")
		filePath := filepath.Join("templates", subPath)

		info, err := os.Stat(filePath)
		if err != nil {
			handleError(w, templates["error"], http.StatusNotFound, "Page not found")
			return
		}
		if info.IsDir() {
			handleError(w, templates["error"], http.StatusForbidden, "Access forbidden")
			return
		}

		http.StripPrefix("/static/", http.FileServer(http.Dir("templates"))).ServeHTTP(w, r)
	})

	// Start server
	port := ":8080"
	fmt.Printf("Server started at http://localhost%s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
