package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
)

var db *sql.DB

type Pumpkin struct {
	ID          int             `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Height      decimal.Decimal `json:"height"`
	Weight      decimal.Decimal `json:"weight"`
}

const pageSize int64 = 5

type PumpkinPage struct {
	Data         []Pumpkin `json:"data"`
	CurrentPage  int64     `json:"current_page"`
	NextPage     *int64    `json:"next_page"`
	PreviousPage *int64    `json:"previous_page"`
}

func returnAllPumpkins(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	searchQuery := queryParams.Get("search")
	sortColumn := queryParams.Get("sort_by")
	sortDirection := queryParams.Get("sort_direction")
	page := int64(1)
	if rawPage, exists := queryParams["page"]; exists {
		parsedPage, err := strconv.ParseInt(rawPage[0], 10, 64)
		if err != nil {
			http.Error(w, "Invalid page number", http.StatusBadRequest)
			return
		}
		page = parsedPage
		if page <= 0 {
			http.Error(w, "Page number out of range", http.StatusNotFound)
			return
		}
	}

	if sortColumn != "" {
		validSortColumns := map[string]bool{
			"id":          true,
			"name":        true,
			"description": true,
			"height":      true,
			"weight":      true,
		}
		if !validSortColumns[sortColumn] {
			http.Error(w, "Invalid sort column", http.StatusBadRequest)
			return
		}
	} else {
		sortColumn = "id"
	}

	if sortDirection != "" {
		if sortDirection != "asc" && sortDirection != "desc" {
			http.Error(w, "Invalid sort direction", http.StatusBadRequest)
			return
		}
	} else {
		sortDirection = "asc"
	}

	searchPattern := "%" + searchQuery + "%"
	var totalCount int64
	err := db.QueryRow(
		"SELECT COUNT(*) FROM pumpkin WHERE name LIKE ?",
		searchPattern,
	).Scan(&totalCount)
	if err != nil {
		http.Error(w, "Error counting pumpkins: "+err.Error(), http.StatusInternalServerError)
		return
	}
	totalPages := int64(0)
	if totalCount > 0 {
		totalPages = (totalCount-1)/pageSize + 1
	}
	if page > totalPages {
		http.Error(w, "Page number out of range", http.StatusNotFound)
		return
	}

	offset := (page - 1) * pageSize
	orderBy := fmt.Sprintf("%s %s", sortColumn, sortDirection)
	if sortColumn != "id" {
		orderBy += ", id ASC"
	}
	query := fmt.Sprintf(
		"SELECT id, name, description, height, weight FROM pumpkin WHERE name LIKE ? ORDER BY %s LIMIT ? OFFSET ?",
		orderBy,
	)
	rows, err := db.Query(query, searchPattern, pageSize, offset)
	if err != nil {
		http.Error(w, "Error querying database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	pumpkins := make([]Pumpkin, 0, pageSize)
	for rows.Next() {
		var pumpkin Pumpkin
		err := rows.Scan(
			&pumpkin.ID,
			&pumpkin.Name,
			&pumpkin.Description,
			&pumpkin.Height,
			&pumpkin.Weight,
		)
		if err != nil {
			http.Error(w, "Error scanning row"+err.Error(), http.StatusInternalServerError)
			return
		}
		pumpkins = append(pumpkins, pumpkin)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Error reading rows"+err.Error(), http.StatusInternalServerError)
		return
	}

	response := PumpkinPage{
		Data:        pumpkins,
		CurrentPage: page,
	}
	if page < totalPages {
		nextPage := page + 1
		response.NextPage = &nextPage
	}
	if page > 1 {
		previousPage := page - 1
		response.PreviousPage = &previousPage
	}

	fmt.Println("Endpoint Hit: returnAllPumpkins")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func returnSinglePumpkin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	var pumpkin Pumpkin

	row := db.QueryRow("SELECT * FROM pumpkin WHERE id = ?", id)
	err := row.Scan(
		&pumpkin.ID,
		&pumpkin.Name,
		&pumpkin.Description,
		&pumpkin.Height,
		&pumpkin.Weight,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Pumpkin not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Error scanning row"+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pumpkin)
}

func createNewPumpkin(w http.ResponseWriter, r *http.Request) {
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body"+err.Error(), http.StatusBadRequest)
		return
	}
	var pumpkin Pumpkin
	err = json.Unmarshal(reqBody, &pumpkin)
	if err != nil {
		http.Error(w, "Invalid JSON"+err.Error(), http.StatusBadRequest)
		return
	}

	result, err := db.Exec(
		"INSERT INTO pumpkin (name, description, height, weight) VALUES (?, ?, ?, ?)",
		pumpkin.Name,
		pumpkin.Description,
		pumpkin.Height,
		pumpkin.Weight,
	)
	if err != nil {
		http.Error(w, "Error inserting pumpkin into database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, "Error getting last insert ID: "+err.Error(), http.StatusInternalServerError)
		return
	}

	pumpkin.ID = int(id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pumpkin)
}

func deletePumpkin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	_, err := db.Exec(
		"DELETE FROM pumpkin WHERE id = ?",
		id,
	)
	if err != nil {
		http.Error(w, "Error deleting pumpkin from database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

func updatePumpkin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stringId := vars["id"]
	var pumpkin Pumpkin

	pumpkinId, err := strconv.Atoi(stringId)
	if err != nil {
		http.Error(w, "Invalid pumpkin ID"+err.Error(), http.StatusBadRequest)
		return
	}
	pumpkin.ID = pumpkinId

	if err := json.NewDecoder(r.Body).Decode(&pumpkin); err != nil {
		http.Error(w, "Invalid request body"+err.Error(), http.StatusBadRequest)
		return
	}

	result, err := db.Exec(
		"UPDATE pumpkin SET name = ?, description = ?, height = ?, weight = ? WHERE id = ?",
		pumpkin.Name,
		pumpkin.Description,
		pumpkin.Height,
		pumpkin.Weight,
		pumpkin.ID,
	)
	if err != nil {
		http.Error(w, "Error updating pumpkin in database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Error checking rows affected: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		http.Error(w, "Pumpkin not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pumpkin)
}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the HomePage!")
	fmt.Println("Endpoint Hit: homePage")
}

func handleRequests() {
	fmt.Println("Starting server on port 10000...")
	myRouter := mux.NewRouter().StrictSlash(true)
	myRouter.HandleFunc("/", homePage)
	myRouter.HandleFunc("/pumpkins", returnAllPumpkins)
	myRouter.HandleFunc("/pumpkin", createNewPumpkin).Methods("POST")
	myRouter.HandleFunc("/pumpkin/{id}", deletePumpkin).Methods("DELETE")
	myRouter.HandleFunc("/pumpkin/{id}", updatePumpkin).Methods("PUT")
	myRouter.HandleFunc("/pumpkin/{id}", returnSinglePumpkin)
	log.Fatal(http.ListenAndServe(":10000", myRouter))
}

func main() {
	// Load environment variables from .env file.
	envErr := godotenv.Load()
	if envErr != nil {
		log.Fatal("Error loading .env file")
	}
	// Capture connection properties.
	cfg := mysql.NewConfig()
	cfg.User = os.Getenv("DBUSER")
	cfg.Passwd = os.Getenv("DBPASS")
	cfg.DBName = os.Getenv("DBNAME")
	cfg.Net = "tcp"
	cfg.Addr = "127.0.0.1:3306"

	// Get a database handle.
	var err error
	db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}

	// Test the connection to the database.
	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}
	fmt.Println("Connected!")

	// Start the HTTP server and handle requests.
	handleRequests()
}
