package main

import (
	"os"
	"fmt"
	"log"
	"time"
	"strings"
	"net/http"
	"sync/atomic"
	"database/sql"
	"encoding/json"
	_ "github.com/lib/pq"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/iavishagupta/chirpy/internal/auth"
	"github.com/iavishagupta/chirpy/internal/database"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	db             *database.Queries
	platform        string
}

//HEALTHZ
func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

//MIDDLEWARE
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

//GET HITS
func (cfg *apiConfig) getHits(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`<html>
								<body>
									<h1>Welcome, Chirpy Admin</h1>
									<p>Chirpy has been visited %d times!</p>
								</body>
								</html>`, cfg.fileServerHits.Load())))
								}

//RESET HITS
func (cfg *apiConfig) resetHits(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		respondWithError(w, http.StatusForbidden, "Forbidden", nil)
		return
	}
	err := cfg.db.DeleteAllUsers(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't delete users", err)
		return
	}

	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	cfg.fileServerHits.Store(0)
	w.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileServerHits.Load())))
}

//RESPOND WITH ERROR
func respondWithError(w http.ResponseWriter, code int, msg string, err error) {
	if err != nil {
		log.Println(err)
	}
	if code > 499 {
		log.Printf("Responding with 5XX error: %s", msg)
	}
	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, code, errorResponse{
		Error: msg,
	})
}

//RESPOND WITH JSON
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(code)
	w.Write(dat)
}

//REMOVE PROFANES
func removeProfanes(body string) string{
	words := strings.Split(body, " ")
	for i, word := range words {
		if (strings.ToLower(word) == "kerfuffle") || (strings.ToLower(word) == "sharbert") || (strings.ToLower(word) == "fornax"){
			words[i] = "****"
		}
	}

	fullStr := strings.Join(words, " ")
	return fullStr
}

//VALIDATE CHIRPS
func (cfg *apiConfig)handlerChirpsValidate(w http.ResponseWriter, r *http.Request) {
	type Chirp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}

	type inputChirp struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}

	input := inputChirp{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}

	params := database.AddChirpsParams{
		Body:   input.Body,
		UserID: input.UserID,
	}

	const maxChirpLength = 140
	if len(params.Body) > maxChirpLength {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long", nil)
		return
	}
	params.Body = removeProfanes(params.Body)
	chirp, err := cfg.db.AddChirps(r.Context(), params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't add Chirps", err)
		return
	}
	respondWithJSON(w, 201, Chirp{
						ID:        chirp.ID,
						CreatedAt: chirp.CreatedAt,
						UpdatedAt: chirp.UpdatedAt,
						Body:      chirp.Body,
						UserID:    chirp.UserID,
					})
}

//CREATE USER
func (cfg *apiConfig)createUser(w http.ResponseWriter, r *http.Request){
	type AddUser struct{
		Password string `json:"password"`
		Email string `json:"email"`
	}

	type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	}

	new_user := AddUser{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&new_user); err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
    	return	
	}

	hashedPassword, err := auth.HashPassword(new_user.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't hash password", err)
		return
	}

	user_res, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		Email:          new_user.Email,
		HashedPassword: hashedPassword,
	})

	user := User{
		ID: user_res.ID,
		CreatedAt: user_res.CreatedAt.Time,
		UpdatedAt: user_res.UpdatedAt.Time,
		Email: user_res.Email,
	}

	respondWithJSON(w, http.StatusCreated, user)
}

//GET ALL CHIRPS
func (cfg *apiConfig)ReturnAllChirps(w http.ResponseWriter, r *http.Request){
	type Chirp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	outputChirps, err := cfg.db.GetAllChirps(r.Context()) 
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't return chirps", err)
	}
	chirps := []Chirp{}
	for _, c := range outputChirps {
		chirps = append(chirps, Chirp{
			ID:        c.ID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Body:      c.Body,
			UserID:    c.UserID,
		})
	}

respondWithJSON(w, http.StatusOK, chirps)
}

func (cfg *apiConfig)HandlerGetChirp(w http.ResponseWriter, r *http.Request){
	type Chirp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	chirp_id := r.PathValue("chirpID")

	uuid_, err := uuid.Parse(chirp_id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse ChirpID", err)
		return
	}

	chirp, err := cfg.db.GetChirp(r.Context(), uuid_)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusNotFound, "Chirp not found", err)
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Couldn't get chirp", err)
		return
	}

	respondWithJSON(w, 200, Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		})

}

func(cfg *apiConfig)loginUser(w http.ResponseWriter, r *http.Request){
	type LoginUser struct{
		Password string `json:"password"`
		Email string `json:"email"`
	}

	var user LoginUser
	decoder := json.NewDecoder(r.Body)
	if err:=decoder.Decode(&user); err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode credentials", err)
		return
	}

	db_cred, err := cfg.db.AuthUser(r.Context(), user.Email)
	if err != nil{
		respondWithError(w, 401, "Unauthorized", err)
		return
	}

	ok, err := auth.CheckPasswordHash(user.Password, db_cred.HashedPassword)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Can't authenticate", err)
		return
	}
	if !ok{
		respondWithError(w, 401, "Unauthorized", err)
		return
	}

	respondWithJSON(w, 200, struct{ID uuid.UUID`json:"id"`
								   CreatedAt time.Time`json:"created_at"`
								   UpdatedAt time.Time`json:"updated_at"`
								   Email string`json:"email"`
								   }{
									ID: db_cred.ID,
									CreatedAt: db_cred.CreatedAt.Time,
									UpdatedAt: db_cred.UpdatedAt.Time,
									Email: db_cred.Email,
									})
}

func main() {
	const filepathRoot = "."
	const port = "8080"
	
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL must be set")
	}

	dbConn, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening database: %s", err)
	}
	dbQueries := database.New(dbConn)

	apiCfg := apiConfig{
		fileServerHits: atomic.Int32{},
		db:             dbQueries,
		platform:       os.Getenv("PLATFORM"),
	}

	mux := http.NewServeMux()

	handler := http.StripPrefix("/app/", http.FileServer(http.Dir(filepathRoot)))

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))
	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	mux.HandleFunc("POST /api/chirps",apiCfg.handlerChirpsValidate)   
	mux.HandleFunc("GET /admin/metrics", apiCfg.getHits)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetHits)
	mux.HandleFunc("POST /api/users", apiCfg.createUser)
	mux.HandleFunc("GET /api/chirps", apiCfg.ReturnAllChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.HandlerGetChirp)
	mux.HandleFunc("POST /api/login", apiCfg.loginUser)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	log.Fatal(srv.ListenAndServe())
}